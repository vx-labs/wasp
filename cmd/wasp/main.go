package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	pprof "net/http/pprof"
	"os"
	"os/signal"
	"path"
	"syscall"
	"time"

	"github.com/hashicorp/memberlist"
	"github.com/vx-labs/cluster"
	"github.com/vx-labs/wasp/v4/wasp/ack"
	"github.com/vx-labs/wasp/v4/wasp/distributed"
	"github.com/vx-labs/wasp/v4/wasp/messages"

	"github.com/spf13/viper"
	"github.com/vx-labs/mqtt-protocol/packet"
	"github.com/vx-labs/wasp/v4/async"
	"github.com/vx-labs/wasp/v4/rpc"
	"github.com/vx-labs/wasp/v4/vaultacme"
	"github.com/vx-labs/wasp/v4/wasp"
	"github.com/vx-labs/wasp/v4/wasp/stats"
	"github.com/vx-labs/wasp/v4/wasp/taps"
	"github.com/vx-labs/wasp/v4/wasp/transport"
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
	messageLog, err := messages.New(path.Join(config.GetString("data-dir"), "published-messages"))
	if err != nil {
		panic(err)
	}
	operations := async.NewOperations(ctx, wasp.L(ctx))

	var clusterMultiNode cluster.MultiNode

	bcast := &memberlist.TransmitLimitedQueue{
		RetransmitMult: 3,
		NumNodes: func() int {
			if clusterMultiNode == nil {
				return 1
			}
			return clusterMultiNode.Gossip().MemberCount()
		},
	}

	healthServer := health.NewServer()
	healthServer.Resume()
	cancelCh := make(chan struct{})
	state := wasp.NewState(id)
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

	auditRecorder, err := getAuditRecorder(ctx, rpcDialer, config, wasp.L(ctx))
	if err != nil {
		wasp.L(ctx).Fatal("failed to create audit recorder", zap.Error(err))
	}

	dstate := distributed.NewState(id, bcast, auditRecorder)

	publishDistributor := &wasp.PublishDistributor{
		ID:      id,
		State:   dstate.Subscriptions(),
		Storage: messageLog,
		Logger:  wasp.L(ctx),
	}

	clusterListener, err := net.Listen("tcp", net.JoinHostPort("::", fmt.Sprintf("%d", config.GetInt("raft-port"))))
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

	memberManager := wasp.NewNodeMemberManager(id, messageLog, dstate)

	clusterMultiNode = cluster.NewMultiNode(cluster.NodeConfig{
		ID:            id,
		ServiceName:   "wasp",
		Version:       BuiltVersion,
		DataDirectory: config.GetString("data-dir"),
		RaftConfig: cluster.RaftConfig{
			Network: cluster.NetworkConfig{
				AdvertizedHost: config.GetString("raft-advertized-address"),
				AdvertizedPort: config.GetInt("raft-advertized-port"),
				ListeningPort:  config.GetInt("raft-port"),
			},
		},
		GossipConfig: cluster.GossipConfig{
			JoinList:                 joinList,
			DistributedStateDelegate: dstate.Distributor(),
			NodeEventDelegate:        memberManager,
			Network: cluster.NetworkConfig{
				AdvertizedHost: config.GetString("serf-advertized-address"),
				AdvertizedPort: config.GetInt("serf-advertized-port"),
				ListeningPort:  config.GetInt("serf-port"),
			},
		},
	}, rpcDialer, server, wasp.L(ctx))

	publishDistributor.Transport = clusterMultiNode

	mqttServer := wasp.NewMQTTServer(dstate, state, messageLog, publishDistributor, clusterMultiNode)
	mqttServer.Serve(server)

	operations.Run("cluster listener", func(ctx context.Context) {
		err := server.Serve(clusterListener)
		if err != nil {
			panic(err)
		}
	})

	operations.Run("audit publisher", func(ctx context.Context) {
		err := auditRecorder.Consume(ctx, func(timestamp int64, tenant, service, eventKind string, payload map[string]string) {
			attributes := map[string]string{}
			for k, v := range payload {
				attributes[k] = v
			}
			attributes["mountpoint"] = tenant
			data := map[string]interface{}{
				"timestamp":  timestamp,
				"service":    service,
				"kind":       eventKind,
				"attributes": attributes,
			}

			buf, _ := json.Marshal(data)
			err := messageLog.Append(&packet.Publish{
				Header:  &packet.Header{Qos: 1},
				Topic:   []byte(fmt.Sprintf("%s/$SYS/_audit/events", tenant)),
				Payload: buf,
			})
			if err != nil {
				wasp.L(ctx).Info("audit event processing failed", zap.Error(err))
			}
		})
		if err != nil {
			panic(err)
		}
	})

	loadedTaps := []taps.Tap{}
	if remote := config.GetString("syslog-tap-address"); remote != "" {
		operations.Run("syslog tap", func(ctx context.Context) {
			tap, err := taps.Syslog(ctx, remote)
			if err != nil {
				wasp.L(ctx).Warn("failed to start syslog tap", zap.Error(err))
				return
			}
			loadedTaps = append(loadedTaps, tap)
		})
	}
	if target := config.GetString("nest-tap-address"); target != "" {
		operations.Run("nest tap", func(ctx context.Context) {
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
			wasp.L(ctx).Debug("nest tap started")
			loadedTaps = append(loadedTaps, tap)
		})
	}
	inflights := ack.NewQueue()
	writer := wasp.NewWriter(id, dstate.Subscriptions(), state, inflights)

	operations.Run("publish distributor", wasp.SchedulePublishes(id, writer, messageLog))
	operations.Run("publish writer", func(ctx context.Context) {
		err := writer.Run(ctx, messageLog)
		if err != nil {
			wasp.L(ctx).Fatal("publish writer failed", zap.Error(err))
		}
	})
	packetProcessor := wasp.NewPacketProcessor(state, dstate, writer, func(sender string, publish *packet.Publish, cb func(publish *packet.Publish)) {
		for _, tap := range loadedTaps {
			err := tap(ctx, sender, publish)
			if err != nil {
				wasp.L(ctx).Warn("failed to run tap", zap.Error(err))
			}
		}
		if publish.Header.Retain {
			if publish.Payload == nil || len(publish.Payload) == 0 {
				err = dstate.Topics().Delete(publish.Topic)
			} else {
				err = dstate.Topics().Set(publish)
			}
			if err != nil {
				wasp.L(ctx).Warn("failed to retain message", zap.Error(err))
			}
			publish.Header.Retain = false
		}
		err := publishDistributor.Distribute(ctx, publish)
		if err != nil {
			wasp.L(ctx).Warn("failed to distribute message", zap.Error(err))
		}
		cb(publish)
	}, inflights)
	operations.Run("packet processor", packetProcessor.Run)

	connManager := wasp.NewConnectionManager(authHandler, state, dstate, writer, packetProcessor, inflights)
	operations.Run("connection manager", connManager.Run)

	handler := func(m transport.Metadata) error {
		connManager.Setup(ctx, m)
		return nil
	}
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
		tlsConfig.ClientAuth = tls.RequestClientCert

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
	connManager.DisconnectClients(ctx)
	wasp.L(ctx).Debug("client connections closed")
	clusterMultiNode.Shutdown()
	healthServer.Shutdown()
	wasp.L(ctx).Debug("health server stopped")
	server.GracefulStop()
	wasp.L(ctx).Debug("rpc server stopped")
	clusterListener.Close()
	wasp.L(ctx).Debug("rpc listener stopped")

	cancel()
	operations.Wait()
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
