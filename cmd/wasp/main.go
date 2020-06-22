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
	"github.com/vx-labs/wasp/cluster"
	"github.com/vx-labs/wasp/wasp/auth"
	"github.com/vx-labs/wasp/wasp/messages"

	"github.com/spf13/viper"
	"github.com/vx-labs/mqtt-protocol/packet"
	"github.com/vx-labs/wasp/async"
	"github.com/vx-labs/wasp/cluster/raft"
	"github.com/vx-labs/wasp/rpc"
	"github.com/vx-labs/wasp/vaultacme"
	"github.com/vx-labs/wasp/wasp"
	"github.com/vx-labs/wasp/wasp/fsm"
	"github.com/vx-labs/wasp/wasp/stats"
	"github.com/vx-labs/wasp/wasp/taps"
	"github.com/vx-labs/wasp/wasp/transport"
	"go.uber.org/zap"
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
	cancelCh := make(chan struct{})
	publishes := make(chan *packet.Publish, 20)
	commandsCh := make(chan raft.Command)
	state := wasp.NewState()
	if config.GetInt("raft-bootstrap-expect") > 1 {
		if config.GetString("rpc-tls-certificate-file") == "" || config.GetString("rpc-tls-private-key-file") == "" {
			wasp.L(ctx).Warn("TLS certificate or private key not provided. GRPC transport security will use a self-signed generated certificate.")
		}
	}
	server := rpc.Server(rpc.ServerConfig{
		VerifyClientCert:            config.GetBool("mtls"),
		TLSCertificateAuthorityPath: config.GetString("rpc-tls-certificate-authority-file"),
		TLSCertificatePath:          config.GetString("rpc-tls-certificate-file"),
		TLSPrivateKeyPath:           config.GetString("rpc-tls-private-key-file"),
	})
	healthpb.RegisterHealthServer(server, healthServer)
	rpcDialer := rpc.GRPCDialer(rpc.ClientConfig{
		InsecureSkipVerify:          config.GetBool("insecure"),
		TLSCertificatePath:          config.GetString("rpc-tls-certificate-file"),
		TLSPrivateKeyPath:           config.GetString("rpc-tls-private-key-file"),
		TLSCertificateAuthorityPath: config.GetString("rpc-tls-certificate-authority-file"),
	})

	authHandler, err := getAuthHandler(ctx, rpcDialer, config)
	if err != nil {
		wasp.L(ctx).Fatal("failed to create authentication handler", zap.Error(err))
	}

	auditRecorder, err := getAuditRecorder(ctx, rpcDialer, config)
	if err != nil {
		wasp.L(ctx).Fatal("failed to create audit recorder", zap.Error(err))
	}
	remotePublishCh := make(chan *packet.Publish, 20)
	stateMachine := fsm.NewFSM(id, state, commandsCh, auditRecorder)
	mqttServer := wasp.NewMQTTServer(state, stateMachine, publishes, remotePublishCh)
	mqttServer.Serve(server)
	clusterListener, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", config.GetInt("raft-port")))
	if err != nil {
		wasp.L(ctx).Fatal("cluster listener failed to start", zap.Error(err))
	}
	joinList := config.GetStringSlice("join-node")
	if config.GetBool("consul-join") {
		discoveryStarted := time.Now()
		consulJoinList, err := findPeers(
			config.GetString("consul-service-name"), config.GetString("consul-service-tag"),
			config.GetInt("raft-bootstrap-expect"))
		if err != nil {
			wasp.L(ctx).Fatal("failed to find other peers on Consul", zap.Error(err))
		}
		wasp.L(ctx).Debug("discovered nodes using Consul",
			zap.Duration("consul_discovery_duration", time.Since(discoveryStarted)), zap.Int("node_count", len(consulJoinList)))
		joinList = append(joinList, consulJoinList...)
	}

	clusterNode := cluster.NewNode(cluster.NodeConfig{
		ID:            id,
		ServiceName:   "wasp",
		DataDirectory: config.GetString("data-dir"),
		GossipConfig: cluster.GossipConfig{
			JoinList: joinList,
			Network: cluster.NetworkConfig{
				AdvertizedHost: config.GetString("serf-advertized-address"),
				AdvertizedPort: config.GetInt("serf-advertized-port"),
				ListeningPort:  config.GetInt("serf-port"),
			},
		},
		RaftConfig: cluster.RaftConfig{
			GetStateSnapshot:  state.MarshalBinary,
			ExpectedNodeCount: config.GetInt("raft-bootstrap-expect"),
			Network: cluster.NetworkConfig{
				AdvertizedHost: config.GetString("raft-advertized-address"),
				AdvertizedPort: config.GetInt("raft-advertized-port"),
				ListeningPort:  config.GetInt("raft-port"),
			},
		},
	}, rpcDialer, server, wasp.L(ctx))
	async.Run(ctx, &wg, func(ctx context.Context) {
		defer wasp.L(ctx).Debug("cluster listener stopped")

		err := server.Serve(clusterListener)
		if err != nil {
			wasp.L(ctx).Fatal("cluster listener crashed", zap.Error(err))
		}
	})
	snapshotter := <-clusterNode.Snapshotter()
	snapshot, err := snapshotter.Load()
	if err != nil {
		wasp.L(ctx).Debug("failed to get state snapshot", zap.Error(err))
	} else {
		err := state.Load(snapshot.Data)
		if err != nil {
			wasp.L(ctx).Warn("failed to load state snapshot", zap.Error(err))
		}
	}
	async.Run(ctx, &wg, func(ctx context.Context) {
		defer wasp.L(ctx).Debug("cluster node stopped")
		clusterNode.Run(ctx)
	})

	async.Run(ctx, &wg, func(ctx context.Context) {
		defer wasp.L(ctx).Debug("command publisher stopped")
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-commandsCh:
				err := clusterNode.Apply(event.Ctx, event.Payload)
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
		defer wasp.L(ctx).Debug("command processor stopped")
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-clusterNode.Commits():
				if event.Payload == nil {
					snapshot, err := snapshotter.Load()
					if err != nil {
						wasp.L(ctx).Fatal("failed to get snapshot from storage", zap.Error(err))
					}
					err = state.Load(snapshot.Data)
					if err != nil {
						wasp.L(ctx).Fatal("failed to get snapshot from storage", zap.Error(err))
					}
					wasp.L(ctx).Debug("loaded snapshot into state")
				} else {
					stateMachine.Apply(event.Payload)
				}
			}
		}
	})
	messageLog, err := messages.New(messages.Options{
		Path: config.GetString("data-dir"),
		BoltOptions: &bolt.Options{
			Timeout:         0,
			InitialMmapSize: 5 * 1000 * 1000,
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
			defer wasp.L(ctx).Debug("syslog tap stopped")
			wasp.L(ctx).Debug("syslog tap started")
			err = taps.Run(ctx, "tap_syslog", messageLog, tap)
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
			tap, err := taps.GRPC(remote)
			if err != nil {
				wasp.L(ctx).Warn("failed to start nest tap", zap.Error(err))
				return
			}
			defer wasp.L(ctx).Debug("nest tap stopped")
			wasp.L(ctx).Debug("nest tap started")
			err = taps.Run(ctx, "tap_grpc", messageLog, tap)
			if err != nil {
				wasp.L(ctx).Info("nest tap failed", zap.Error(err))
			}
		})
	}
	async.Run(ctx, &wg, func(ctx context.Context) {
		defer wasp.L(ctx).Debug("publish processor stopped")
		messageLog.Consume(ctx, "publish_processor", func(p *packet.Publish) error {
			err := wasp.ProcessPublish(ctx, id, clusterNode, stateMachine, state, true, p)
			if err != nil {
				wasp.L(ctx).Info("publish processing failed", zap.Error(err))
			}
			return err
		})
	})
	async.Run(ctx, &wg, func(ctx context.Context) {
		defer wasp.L(ctx).Debug("remote publish processor stopped")
		for {
			select {
			case <-ctx.Done():
				return
			case p := <-remotePublishCh:
				err := wasp.ProcessPublish(ctx, id, clusterNode, stateMachine, state, false, p)
				if err != nil {
					wasp.L(ctx).Info("remote publish processing failed", zap.Error(err))
				}
			}
		}
	})

	async.Run(ctx, &wg, func(ctx context.Context) {
		defer wasp.L(ctx).Debug("publish storer stopped")
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
			wasp.RunSession(ctx, id, stateMachine, state, m.Channel, publishes,
				func(ctx context.Context, mqtt auth.ApplicationContext) (id string, mountpoint string, err error) {
					principal, err := authHandler.Authenticate(ctx, mqtt, auth.TransportContext{
						Encrypted:       m.Encrypted,
						RemoteAddress:   m.RemoteAddress,
						X509Certificate: nil,
					})
					return principal.ID, principal.MountPoint, err
				})
		}()
		return nil
	}
	<-clusterNode.Ready()
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
	if !config.GetBool("headless") {
		defaultIP := localPrivateHost()
		fmt.Printf("🐝 Wasp is ready and serving !🐝\n")
		fmt.Println()
		if len(listeners) > 0 {
			fmt.Printf("🔌 You can connect to Wasp using:\n")
			for _, listener := range listeners {
				switch listener.name {
				case "tcp":
					fmt.Printf("  • MQTT on address %s:%d\n", defaultIP, listener.port)
				case "tls":
					fmt.Printf("  • MQTT-over-TLS on address %s:%d\n", defaultIP, listener.port)
				case "ws":
					fmt.Printf("  • WebSocket on address %s:%d\n", defaultIP, listener.port)
				case "wss":
					fmt.Printf("  • WebSocket-over-TLS on address %s:%d\n", defaultIP, listener.port)
				}
			}
		} else {
			fmt.Printf("No listener were enabled, so you won't be able to connect to this node using MQTT.\n")
		}
		fmt.Println("")
		fmt.Println("⚡️ You can stop the service by pressing Ctrl+C.")
	}
	select {
	case <-sigc:
	case <-cancelCh:
	}
	if !config.GetBool("headless") {
		fmt.Println()
		fmt.Printf("⌛Shutting down Wasp..\n")
	}
	wasp.L(ctx).Debug("wasp shutdown initiated")
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
	err = clusterNode.Shutdown()
	if err != nil {
		wasp.L(ctx).Error("failed to leave cluster", zap.Error(err))
	} else {
		wasp.L(ctx).Debug("cluster left")
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

	err = messageLog.Close()
	if err != nil {
		wasp.L(ctx).Error("failed to close message log", zap.Error(err))
	} else {
		wasp.L(ctx).Debug("message log closed")
	}
	wasp.L(ctx).Info("wasp successfully stopped")
	if !config.GetBool("headless") {
		fmt.Printf("👋 Shutdown complete.\n")
	}
}
