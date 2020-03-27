package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/spf13/viper"
	"github.com/vx-labs/mqtt-protocol/packet"
	"github.com/vx-labs/wasp/vaultacme"
	"github.com/vx-labs/wasp/wasp"
	"github.com/vx-labs/wasp/wasp/api"
	"github.com/vx-labs/wasp/wasp/async"
	"github.com/vx-labs/wasp/wasp/fsm"
	"github.com/vx-labs/wasp/wasp/membership"
	"github.com/vx-labs/wasp/wasp/raft"
	"github.com/vx-labs/wasp/wasp/rpc"
	"github.com/vx-labs/wasp/wasp/transport"
	"go.uber.org/zap"
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

	wg := sync.WaitGroup{}
	stateLoaded := make(chan struct{})
	cancelCh := make(chan struct{})
	publishes := make(chan *packet.Publish, 20)
	commandsCh := make(chan raft.Command)
	state := wasp.NewState()

	if config.GetString("rpc-tls-certificate-file") == "" || config.GetString("rpc-tls-private-key-file") == "" {
		wasp.L(ctx).Warn("TLS certificate or private key not provided. GRPC transport security will be disabled.")
	}
	server := rpc.Server(rpc.ServerConfig{
		TLSCertificatePath: config.GetString("rpc-tls-certificate-file"),
		TLSPrivateKeyPath:  config.GetString("rpc-tls-private-key-file"),
	})
	api.RegisterNodeServer(server, &nodeRPCServer{cancel: cancelCh})
	rpcDialer := rpc.GRPCDialer(rpc.ClientConfig{
		TLSCertificateAuthorityPath: config.GetString("rpc-tls-certificate-authority-file"),
	})
	remotePublishCh := make(chan *packet.Publish, 20)
	mqttServer := rpc.NewMQTTServer(remotePublishCh)
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
		wasp.L(ctx),
	)
	raftAddress := fmt.Sprintf("%s:%d", config.GetString("raft-advertized-address"), config.GetInt("raft-advertized-port"))
	mesh.UpdateMetadata(membership.EncodeMD(id,
		raftAddress,
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
	raftNode := raft.NewNode(raftConfig, wasp.L(ctx))
	raftNode.Serve(server)
	rpcTransport := rpc.NewTransport(raftConfig.NodeID,
		fmt.Sprintf("%s:%d", config.GetString("raft-advertized-address"), config.GetInt("raft-advertized-port")), raftNode, rpcDialer)

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
			peers, err = membership.WaitForNodes(ctx, mesh, expectedCount, api.RaftContext{
				ID:      id,
				Address: raftAddress,
			}, rpcDialer)
			if err != nil {
				if err == ErrExistingClusterFound {
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
			case removed := <-raftNode.Ready():
				if removed {
					wasp.L(ctx).Debug("local node is not a cluster member, will attempt join")
					ticker := time.NewTicker(1 * time.Second)
					defer ticker.Stop()
					for {
						for _, peer := range peers {
							conn, err := rpcDialer(peer.Address)
							if err != nil {
								wasp.L(ctx).Error("failed to dial peer")
								continue
							}
							ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
							_, err = api.NewRaftClient(conn).JoinCluster(ctx, &api.RaftContext{
								ID:      id,
								Address: raftAddress,
							})
							cancel()
							conn.Close()
							if err != nil {
								wasp.L(ctx).Warn("failed to join raft cluster", zap.Error(err))
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
		raftNode.Run(ctx, peers, join, rpcTransport)
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
	stateMachine := fsm.NewFSM(id, state, commandsCh)
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
					event.ErrCh <- ctx.Err()
				case <-event.Ctx.Done():
					event.ErrCh <- event.Ctx.Err()
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
				stateMachine.Apply(event)
			}
		}
	})
	messageLog, err := wasp.NewMessageLog(ctx, config.GetString("data-dir"))
	if err != nil {
		panic(err)
	}
	async.Run(ctx, &wg, func(ctx context.Context) {
		defer wasp.L(ctx).Info("message log gc runner stopped")
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				messageLog.GC()
			}
		}
	})
	async.Run(ctx, &wg, func(ctx context.Context) {
		defer wasp.L(ctx).Info("publish processor stopped")
		messageLog.Consume(ctx, func(p *packet.Publish) {
			err := wasp.ProcessPublish(ctx, id, rpcTransport, stateMachine, state, true, p)
			if err != nil {
				wasp.L(ctx).Info("publish processing failed", zap.Error(err))
			}
		})
	})
	async.Run(ctx, &wg, func(ctx context.Context) {
		defer wasp.L(ctx).Info("remote publish processor stopped")
		for {
			select {
			case <-ctx.Done():
				return
			case p := <-remotePublishCh:
				err := wasp.ProcessPublish(ctx, id, rpcTransport, stateMachine, state, false, p)
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
			wasp.RunSession(ctx, stateMachine, state, m.Channel, publishes)
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
	runProm(config.GetInt("metrics-port"))
	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT)
	select {
	case <-sigc:
	case <-cancelCh:
	}
	wasp.L(ctx).Info("shutting down wasp")
	for _, listener := range listeners {
		listener.listener.Close()
		wasp.L(ctx).Info("listener stopped", zap.String("listener_name", listener.name), zap.Int("listener_port", listener.port))
	}
	err = stateMachine.Shutdown(ctx)
	if err != nil {
		wasp.L(ctx).Error("failed to stop state machine", zap.Error(err))
	}
	err = raftNode.Leave(ctx)
	if err != nil {
		wasp.L(ctx).Error("failed to leave raft cluster", zap.Error(err))
	} else {
		wasp.L(ctx).Info("raft cluster left")
	}
	cancel()
	server.GracefulStop()
	clusterListener.Close()
	wg.Wait()
	err = messageLog.Close()
	if err != nil {
		wasp.L(ctx).Error("failed to close message log", zap.Error(err))
	} else {
		wasp.L(ctx).Info("message log closed")
	}
	wasp.L(ctx).Info("wasp shutdown")
}
