package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	pprof "net/http/pprof"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/boltdb/bolt"
	"github.com/vx-labs/wasp/wasp/messages"

	"github.com/spf13/viper"
	"github.com/vx-labs/mqtt-protocol/packet"
	"github.com/vx-labs/wasp/cluster"
	"github.com/vx-labs/wasp/cluster/membership"
	"github.com/vx-labs/wasp/cluster/raft"
	"github.com/vx-labs/wasp/vaultacme"
	"github.com/vx-labs/wasp/wasp"
	"github.com/vx-labs/wasp/wasp/api"
	"github.com/vx-labs/wasp/wasp/async"
	"github.com/vx-labs/wasp/wasp/fsm"
	"github.com/vx-labs/wasp/wasp/rpc"
	"github.com/vx-labs/wasp/wasp/stats"
	"github.com/vx-labs/wasp/wasp/taps"
	"github.com/vx-labs/wasp/wasp/transport"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

func run(config *viper.Viper) {
	ctx, cancel := context.WithCancel(context.Background())
	ctx = wasp.StoreLogger(ctx, getLogger(config))
	err := os.MkdirAll(config.GetString("data-dir"), 0700)
	if err != nil {
		wasp.L(ctx).Fatal("failed to create data directory", zap.Error(err))
	}
	id, err := loadID(config.GetString("data-dir"))
	if err != nil {
		wasp.L(ctx).Fatal("failed to get node ID", zap.Error(err))
	}
	ctx = wasp.AddFields(ctx, zap.String("hex_node_id", fmt.Sprintf("%x", id)))
	if config.GetBool("pprof") {
		address := fmt.Sprintf("%s:%d", config.GetString("pprof-address"), config.GetInt("pprof-port"))
		go func() {
			mux := http.NewServeMux()
			mux.HandleFunc("/debug/pprof/", pprof.Index)
			mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
			mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
			mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
			mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
			panic(http.ListenAndServe(address, mux))
		}()
		wasp.L(ctx).Info("started pprof", zap.String("pprof_url", fmt.Sprintf("http://%s/", address)))
	}
	healthServer := health.NewServer()
	healthServer.Resume()
	wg := sync.WaitGroup{}
	stateLoaded := make(chan struct{})
	cancelCh := make(chan struct{})
	publishes := make(chan *packet.Publish, 20)
	commandsCh := make(chan raft.Command)
	state := wasp.NewState()

	if config.GetString("rpc-tls-certificate-file") == "" || config.GetString("rpc-tls-private-key-file") == "" {
		wasp.L(ctx).Warn("TLS certificate or private key not provided. GRPC transport security will use a self-signed generated certificate.")
	}
	server := rpc.Server(rpc.ServerConfig{
		VerifyClientCert:            config.GetBool("mtls"),
		TLSCertificateAuthorityPath: config.GetString("rpc-tls-certificate-authority-file"),
		TLSCertificatePath:          config.GetString("rpc-tls-certificate-file"),
		TLSPrivateKeyPath:           config.GetString("rpc-tls-private-key-file"),
	})
	healthpb.RegisterHealthServer(server, healthServer)
	api.RegisterNodeServer(server, rpc.NewNodeRPCServer(cancelCh))
	rpcDialer := rpc.GRPCDialer(rpc.ClientConfig{
		InsecureSkipVerify:          config.GetBool("insecure"),
		TLSCertificatePath:          config.GetString("rpc-tls-certificate-file"),
		TLSPrivateKeyPath:           config.GetString("rpc-tls-private-key-file"),
		TLSCertificateAuthorityPath: config.GetString("rpc-tls-certificate-authority-file"),
	})
	remotePublishCh := make(chan *packet.Publish, 20)
	stateMachine := fsm.NewFSM(id, state, commandsCh)
	mqttServer := rpc.NewMQTTServer(state, stateMachine, publishes, remotePublishCh)
	mqttServer.Serve(server)
	clusterListener, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", config.GetInt("raft-port")))
	if err != nil {
		wasp.L(ctx).Fatal("cluster listener failed to start", zap.Error(err))
	}
	mesh := membership.New(
		id,
		config.GetInt("serf-port"),
		config.GetString("serf-advertized-address"),
		config.GetInt("serf-advertized-port"),
		config.GetInt("raft-advertized-port"),
		rpcDialer,
		wasp.L(ctx),
	)
	rpcAddress := fmt.Sprintf("%s:%d", config.GetString("raft-advertized-address"), config.GetInt("raft-advertized-port"))
	mesh.UpdateMetadata(membership.EncodeMD(id,
		rpcAddress,
	))
	joinList := config.GetStringSlice("join-node")
	if config.GetBool("consul-join") {
		discoveryStarted := time.Now()
		consulJoinList, err := findPeers(
			config.GetString("consul-service-name"), config.GetString("consul-service-tag"),
			config.GetInt("raft-bootstrap-expect"))
		if err != nil {
			wasp.L(ctx).Fatal("failed to find other peers on Consul", zap.Error(err))
		}
		wasp.L(ctx).Info("discovered nodes using Consul",
			zap.Duration("consul_discovery_duration", time.Since(discoveryStarted)), zap.Int("node_count", len(consulJoinList)))
		joinList = append(joinList, consulJoinList...)
	}
	if len(joinList) > 0 {
		joinStarted := time.Now()
		retryTicker := time.NewTicker(3 * time.Second)
		for {
			err = mesh.Join(joinList)
			if err != nil {
				wasp.L(ctx).Warn("failed to join gossip mesh", zap.Error(err))
			} else {
				break
			}
			<-retryTicker.C
		}
		retryTicker.Stop()
		wasp.L(ctx).Info("joined gossip mesh",
			zap.Duration("gossip_join_duration", time.Since(joinStarted)), zap.Strings("gossip_node_list", joinList))
	}
	raftConfig := raft.Config{
		NodeID:      id,
		DataDir:     config.GetString("data-dir"),
		GetSnapshot: state.MarshalBinary,
	}
	raftNode := raft.NewNode(raftConfig, mesh, wasp.L(ctx))
	raftNode.Serve(server)

	async.Run(ctx, &wg, func(ctx context.Context) {
		defer wasp.L(ctx).Info("cluster listener stopped")

		err := server.Serve(clusterListener)
		if err != nil {
			wasp.L(ctx).Fatal("cluster listener crashed", zap.Error(err))
		}
	})
	async.Run(ctx, &wg, func(ctx context.Context) {
		defer wasp.L(ctx).Info("raft node stopped")
		join := false
		peers := raft.Peers{}
		if expectedCount := config.GetInt("raft-bootstrap-expect"); expectedCount > 1 {
			wasp.L(ctx).Debug("waiting for nodes to be discovered", zap.Int("expected_node_count", expectedCount))
			peers, err = mesh.WaitForNodes(ctx, expectedCount, cluster.RaftContext{
				ID:      id,
				Address: rpcAddress,
			}, rpcDialer)
			if err != nil {
				if err == membership.ErrExistingClusterFound {
					wasp.L(ctx).Info("discovered existing raft cluster")
					join = true
				} else {
					wasp.L(ctx).Fatal("failed to discover nodes on gossip mesh", zap.Error(err))
				}
			}
			wasp.L(ctx).Info("discovered nodes on gossip mesh", zap.Int("discovered_node_count", len(peers)))
		} else {
			wasp.L(ctx).Info("skipping raft node discovery: expected node count is below 1", zap.Int("expected_node_count", expectedCount))
		}
		if join {
			wasp.L(ctx).Info("joining raft cluster", zap.Array("raft_peers", peers))
		} else {
			wasp.L(ctx).Info("bootstraping raft cluster", zap.Array("raft_peers", peers))
		}
		go func() {
			defer close(stateLoaded)
			select {
			case <-raftNode.Ready():
				if join && raftNode.IsRemovedFromCluster() {
					wasp.L(ctx).Debug("local node is not a cluster member, will attempt join")
					ticker := time.NewTicker(1 * time.Second)
					defer ticker.Stop()
					for {
						if raftNode.IsLeader() {
							return
						}
						for _, peer := range peers {
							ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
							err := mesh.Call(peer.ID, func(c *grpc.ClientConn) error {
								_, err = cluster.NewRaftClient(c).JoinCluster(ctx, &cluster.RaftContext{
									ID:      id,
									Address: rpcAddress,
								})
								return err
							})
							cancel()
							if err != nil {
								wasp.L(ctx).Debug("failed to join raft cluster, retrying", zap.Error(err))
							} else {
								wasp.L(ctx).Info("joined cluster")
								return
							}
						}
						select {
						case <-ticker.C:
						case <-ctx.Done():
							return
						}
					}
				}
			case <-ctx.Done():
				return
			}
		}()
		raftNode.Run(ctx, peers, join)
	})
	snapshotter := <-raftNode.Snapshotter()

	snapshot, err := snapshotter.Load()
	if err != nil {
		wasp.L(ctx).Warn("failed to get state snapshot", zap.Error(err))
	} else {
		err := state.Load(snapshot.Data)
		if err != nil {
			wasp.L(ctx).Warn("failed to load state snapshot", zap.Error(err))
		}
	}
	async.Run(ctx, &wg, func(ctx context.Context) {
		defer wasp.L(ctx).Info("command publisher stopped")
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-commandsCh:
				err := raftNode.Apply(event.Ctx, event.Payload)
				select {
				case <-ctx.Done():
				case <-event.Ctx.Done():
				case event.ErrCh <- err:
				}
				close(event.ErrCh)
			}
		}
	})
	async.Run(ctx, &wg, func(ctx context.Context) {
		defer wasp.L(ctx).Info("command processor stopped")
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-raftNode.Commits():
				if event == nil {
					snapshot, err := snapshotter.Load()
					if err != nil {
						wasp.L(ctx).Fatal("failed to get snapshot from storage", zap.Error(err))
					}
					err = state.Load(snapshot.Data)
					if err != nil {
						wasp.L(ctx).Fatal("failed to get snapshot from storage", zap.Error(err))
					}
					wasp.L(ctx).Info("loaded snapshot into state")
				} else {
					stateMachine.Apply(event)
				}
			}
		}
	})
	messageLog, err := messages.New(messages.Options{
		Path: config.GetString("data-dir"),
		BoltOptions: &bolt.Options{
			Timeout:         0,
			InitialMmapSize: 5 * 1000 * 1000,
			MmapFlags:       syscall.MAP_POPULATE,
			ReadOnly:        false,
		},
	})
	if err != nil {
		panic(err)
	}
	if remote := config.GetString("syslog-tap-address"); remote != "" {
		async.Run(ctx, &wg, func(ctx context.Context) {
			tap, err := taps.Syslog(ctx, remote)
			if err != nil {
				wasp.L(ctx).Warn("failed to start syslog tap", zap.Error(err))
				return
			}
			defer wasp.L(ctx).Info("syslog tap stopped")
			wasp.L(ctx).Debug("syslog tap started")
			err = taps.Run(ctx, messageLog, tap)
			if err != nil {
				wasp.L(ctx).Info("syslog tap failed", zap.Error(err))
			}
		})
	}
	if target := config.GetString("nest-tap-address"); target != "" {
		async.Run(ctx, &wg, func(ctx context.Context) {
			remote, err := rpcDialer(target)
			if err != nil {
				wasp.L(ctx).Warn("failed to dial nest tap", zap.Error(err))
				return
			}
			tap, err := taps.Nest(remote)
			if err != nil {
				wasp.L(ctx).Warn("failed to start nest tap", zap.Error(err))
				return
			}
			defer wasp.L(ctx).Info("nest tap stopped")
			wasp.L(ctx).Debug("nest tap started")
			err = taps.Run(ctx, messageLog, tap)
			if err != nil {
				wasp.L(ctx).Info("nest tap failed", zap.Error(err))
			}
		})
	}
	async.Run(ctx, &wg, func(ctx context.Context) {
		defer wasp.L(ctx).Info("publish processor stopped")
		messageLog.Consume(ctx, 0, func(p *packet.Publish) error {
			err := wasp.ProcessPublish(ctx, id, mesh, stateMachine, state, true, p)
			if err != nil {
				wasp.L(ctx).Info("publish processing failed", zap.Error(err))
			}
			return err
		})
	})
	async.Run(ctx, &wg, func(ctx context.Context) {
		defer wasp.L(ctx).Info("remote publish processor stopped")
		for {
			select {
			case <-ctx.Done():
				return
			case p := <-remotePublishCh:
				err := wasp.ProcessPublish(ctx, id, mesh, stateMachine, state, false, p)
				if err != nil {
					wasp.L(ctx).Info("remote publish processing failed", zap.Error(err))
				}
			}
		}
	})

	async.Run(ctx, &wg, func(ctx context.Context) {
		defer wasp.L(ctx).Info("publish storer stopped")
		buf := make([]*packet.Publish, 0, 100)
		ticker := time.NewTicker(20 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if len(buf) > 0 {
					err := wasp.StorePublish(messageLog, buf)
					if err != nil {
						wasp.L(ctx).Error("publish storing failed", zap.Error(err))
					}
				}
				buf = buf[:0]
			case p := <-publishes:
				buf = append(buf, p)
				if len(buf) == 100 {
					err := wasp.StorePublish(messageLog, buf)
					if err != nil {
						wasp.L(ctx).Error("publish storing failed", zap.Error(err))
					}
					buf = buf[:0]
				}
			}
		}
	})
	handler := func(m transport.Metadata) error {
		go func() {
			ctx, cancel := context.WithCancel(ctx)
			defer cancel()
			ctx = wasp.AddFields(ctx, zap.String("transport", m.Name), zap.String("remote_address", m.RemoteAddress))
			wasp.RunSession(ctx, id, stateMachine, state, m.Channel, publishes)
		}()
		return nil
	}
	<-stateLoaded
	listeners := []listenerConfig{}
	if port := config.GetInt("tcp-port"); port > 0 {
		ln, err := transport.NewTCPTransport(port, handler)
		if err != nil {
			wasp.L(ctx).Error("failed to start listener", zap.String("listener_name", "tcp"), zap.Error(err))
		} else {
			listeners = append(listeners, listenerConfig{name: "tcp", port: port, listener: ln})
		}
	}
	if port := config.GetInt("ws-port"); port > 0 {
		ln, err := transport.NewWSTransport(port, handler)
		if err != nil {
			wasp.L(ctx).Error("failed to start listener", zap.String("listener_name", "ws"), zap.Error(err))
		} else {
			listeners = append(listeners, listenerConfig{name: "ws", port: port, listener: ln})
		}
	}
	var tlsConfig *tls.Config
	if config.GetBool("use-vault") {
		tlsConfig, err = vaultacme.GetConfig(ctx, config.GetString("tls-cn"), wasp.L(ctx))
		if err != nil {
			wasp.L(ctx).Fatal("failed to get TLS certificate from ACME", zap.Error(err))
		}

		if port := config.GetInt("wss-port"); port > 0 {
			ln, err := transport.NewWSSTransport(tlsConfig, port, handler)
			if err != nil {
				wasp.L(ctx).Error("failed to start listener", zap.String("listener_name", "wss"), zap.Error(err))
			} else {
				listeners = append(listeners, listenerConfig{name: "wss", port: port, listener: ln})
			}
		}
		if port := config.GetInt("tls-port"); port > 0 {
			ln, err := transport.NewTLSTransport(tlsConfig, port, handler)
			if err != nil {
				wasp.L(ctx).Error("failed to start listener", zap.String("listener_name", "tls"), zap.Error(err))
			} else {
				listeners = append(listeners, listenerConfig{name: "tls", port: port, listener: ln})
			}
		}
	}
	for _, listener := range listeners {
		wasp.L(ctx).Debug("listener started", zap.String("listener_name", listener.name), zap.Int("listener_port", listener.port))
	}
	go stats.ListenAndServe(config.GetInt("metrics-port"))

	healthServer.SetServingStatus("mqtt", healthpb.HealthCheckResponse_SERVING)
	healthServer.SetServingStatus("node", healthpb.HealthCheckResponse_SERVING)
	healthServer.SetServingStatus("rpc", healthpb.HealthCheckResponse_SERVING)
	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT)
	select {
	case <-sigc:
	case <-cancelCh:
	}
	wasp.L(ctx).Info("wasp shutdown initiated")
	for _, listener := range listeners {
		listener.listener.Close()
	}
	wasp.L(ctx).Debug("mqtt listeners stopped")
	err = stateMachine.Shutdown(ctx)
	if err != nil {
		wasp.L(ctx).Error("failed to stop state machine", zap.Error(err))
	} else {
		wasp.L(ctx).Debug("state machine stopped")
	}
	err = raftNode.Leave(ctx)
	if err != nil {
		wasp.L(ctx).Error("failed to leave raft cluster", zap.Error(err))
	} else {
		wasp.L(ctx).Debug("raft cluster left")
	}
	healthServer.Shutdown()
	wasp.L(ctx).Debug("health server stopped")
	server.GracefulStop()
	wasp.L(ctx).Debug("rpc server stopped")
	clusterListener.Close()
	wasp.L(ctx).Debug("rpc listener stopped")
	cancel()
	wg.Wait()
	wasp.L(ctx).Debug("asynchronous operations stopped")
	mesh.Shutdown()
	wasp.L(ctx).Debug("mesh stopped")
	err = messageLog.Close()
	if err != nil {
		wasp.L(ctx).Error("failed to close message log", zap.Error(err))
	} else {
		wasp.L(ctx).Debug("message log closed")
	}
	wasp.L(ctx).Info("wasp successfully stopped")
}
