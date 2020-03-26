package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	consulapi "github.com/hashicorp/consul/api"

	"github.com/hashicorp/memberlist"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"google.golang.org/grpc"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/spf13/cobra"
	"github.com/vx-labs/mqtt-protocol/packet"
	"github.com/vx-labs/wasp/vaultacme"
	"github.com/vx-labs/wasp/wasp"
	"github.com/vx-labs/wasp/wasp/api"
	"github.com/vx-labs/wasp/wasp/fsm"
	"github.com/vx-labs/wasp/wasp/membership"
	"github.com/vx-labs/wasp/wasp/raft"
	"github.com/vx-labs/wasp/wasp/rpc"
	"github.com/vx-labs/wasp/wasp/transport"
)

var (
	ErrExistingClusterFound = errors.New("existing cluster found")
)

type listenerConfig struct {
	name     string
	port     int
	listener net.Listener
}

func runProm(port int) {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	go http.ListenAndServe(fmt.Sprintf("0.0.0.0:%d", port), mux)
}

type MemberlistMemberProvider interface {
	Members() []*memberlist.Node
}

type MemberMetadata struct {
	RaftAddress string `json:"raft_address"`
	ID          uint64 `json:"id"`
}

func localPrivateHost() string {
	ifaces, err := net.Interfaces()
	if err != nil {
		panic(err)
	}

	for _, v := range ifaces {
		if v.Flags&net.FlagLoopback != net.FlagLoopback && v.Flags&net.FlagUp == net.FlagUp {
			h := v.HardwareAddr.String()
			if len(h) == 0 {
				continue
			} else {
				addresses, _ := v.Addrs()
				if len(addresses) > 0 {
					ip := addresses[0]
					if ipnet, ok := ip.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
						if ipnet.IP.To4() != nil {
							return ipnet.IP.String()
						}
					}
				}
			}
		}
	}
	panic("could not find a valid network interface")
}

func decodeMD(buf []byte) (MemberMetadata, error) {
	md := MemberMetadata{}
	return md, json.Unmarshal(buf, &md)
}
func encodeMD(id uint64, raftAddress string) []byte {
	md := MemberMetadata{
		ID:          id,
		RaftAddress: raftAddress,
	}
	p, _ := json.Marshal(md)
	return p
}

func waitForNodes(ctx context.Context, mesh MemberlistMemberProvider, expectedNumber int, localContext api.RaftContext, rpcDialer rpc.Dialer) ([]raft.Peer, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	for {
		nodes := mesh.Members()
		for idx := range nodes {
			md, err := decodeMD(nodes[idx].Meta)
			if err != nil {
				continue
			}
			conn, err := rpcDialer(md.RaftAddress,
				grpc.WithBlock(), grpc.WithTimeout(300*time.Millisecond))
			if err != nil {
				if err == context.DeadlineExceeded {
					continue
				} else {
					return nil, err
				}
			}
			ctx, cancel = context.WithTimeout(ctx, 500*time.Millisecond)
			clusterPeers, err := api.NewRaftClient(conn).JoinCluster(ctx, &localContext)
			cancel()
			if err == nil {
				peers := []raft.Peer{}
				for _, clusterPeer := range clusterPeers.GetPeers() {
					peers = append(peers, raft.Peer{ID: clusterPeer.ID, Address: clusterPeer.Address})
				}
				return peers, ErrExistingClusterFound
			}
		}
		if len(nodes) >= expectedNumber {
			peers := make([]raft.Peer, len(nodes))
			for idx := range peers {
				md, err := decodeMD(nodes[idx].Meta)
				if err != nil {
					return nil, err
				}
				peers[idx] = raft.Peer{Address: md.RaftAddress, ID: md.ID}
			}
			return peers, nil
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
		}
	}
}

func findPeers(name, tag string, minimumCount int) ([]string, error) {
	config := consulapi.DefaultConfig()
	config.HttpClient = http.DefaultClient
	client, err := consulapi.NewClient(config)
	if err != nil {
		return nil, err
	}
	var idx uint64
	for {
		services, meta, err := client.Catalog().Service(name, tag, &consulapi.QueryOptions{
			WaitIndex: idx,
			WaitTime:  10 * time.Second,
		})
		if err != nil {
			return nil, err
		}
		idx = meta.LastIndex
		if len(services) < minimumCount {
			continue
		}
		out := make([]string, len(services))
		for idx := range services {
			out[idx] = fmt.Sprintf("%s:%d", services[idx].ServiceAddress, services[idx].ServicePort)
		}
		return out, nil
	}
}

type nodeRPCServer struct {
	cancel chan<- struct{}
}

func (n *nodeRPCServer) Shutdown(ctx context.Context, _ *api.ShutdownRequest) (*api.ShutdownResponse, error) {
	n.cancel <- struct{}{}
	return &api.ShutdownResponse{}, nil
}
func main() {
	cancelCh := make(chan struct{})
	config := viper.New()
	config.SetEnvPrefix("WASP")
	config.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	config.AutomaticEnv()
	cmd := &cobra.Command{
		Use: "wasp",
		PreRun: func(cmd *cobra.Command, _ []string) {
			config.BindPFlags(cmd.Flags())
			if !cmd.Flags().Changed("serf-advertized-port") {
				config.Set("serf-advertized-port", config.Get("serf-port"))
			}
			if !cmd.Flags().Changed("raft-advertized-port") {
				config.Set("raft-advertized-port", config.Get("raft-port"))
			}

		},
		Run: func(cmd *cobra.Command, _ []string) {
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
			clusterListener, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", config.GetInt("raft-port")))
			if err != nil {
				wasp.L(ctx).Fatal("cluster listener failed to start", zap.Error(err))
			}
			membership := membership.New(
				id,
				config.GetInt("serf-port"),
				config.GetString("serf-advertized-address"),
				config.GetInt("serf-advertized-port"),
				wasp.L(ctx),
			)
			raftAddress := fmt.Sprintf("%s:%d", config.GetString("raft-advertized-address"), config.GetInt("raft-advertized-port"))
			membership.UpdateMetadata(encodeMD(id,
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
					err = membership.Join(joinList)
					if err != nil {
						wasp.L(ctx).Warn("failed to join cluster", zap.Error(err))
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
				Join:        false,
				GetSnapshot: state.MarshalBinary,
				ProposeC:    commandsCh,
			}
			if expectedCount := config.GetInt("raft-bootstrap-expect"); expectedCount > 1 {
				wasp.L(ctx).Debug("waiting for nodes to be discovered", zap.Int("expected_node_count", expectedCount))
				raftConfig.Peers, err = waitForNodes(ctx, membership, expectedCount, api.RaftContext{
					ID:      id,
					Address: raftAddress,
				}, rpcDialer)
				if err != nil {
					if err == ErrExistingClusterFound {
						wasp.L(ctx).Info("discovered existing raft cluster")
						raftConfig.Join = true
					} else {
						wasp.L(ctx).Fatal("failed to discover nodes on gossip mesh", zap.Error(err))
					}
				}
				wasp.L(ctx).Info("discovered nodes on gossip mesh", zap.Int("discovered_node_count", len(raftConfig.Peers)))
			} else {
				wasp.L(ctx).Info("skipping raft node discovery: expected node count is below 1", zap.Int("expected_node_count", expectedCount))
			}
			if raftConfig.Join {
				wasp.L(ctx).Info("joining raft cluster", zap.Array("raft_peers", raftConfig.Peers))
			} else {
				wasp.L(ctx).Info("bootstraping raft cluster", zap.Array("raft_peers", raftConfig.Peers))
			}

			raftNode := raft.NewNode(raftConfig, wasp.L(ctx))
			rpcTransport := rpc.NewTransport(raftConfig.NodeID,
				fmt.Sprintf("%s:%d", config.GetString("raft-advertized-address"), config.GetInt("raft-advertized-port")), raftNode, rpcDialer)
			rpcTransport.Serve(server)
			raftNode.Start(rpcTransport)
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

			wg.Add(1)
			go func() {
				defer func() {
					wasp.L(ctx).Info("command processor stopped")
					wg.Done()
				}()
				for {
					select {
					case <-ctx.Done():
						return
					case event := <-raftNode.Commits():
						stateMachine.Apply(event)
					}
				}
			}()
			messageLog, err := wasp.NewMessageLog(ctx, config.GetString("data-dir"))
			if err != nil {
				panic(err)
			}
			wg.Add(1)
			go func() {
				defer func() {
					wasp.L(ctx).Info("message log gc runner stopped")
					wg.Done()
				}()
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
			}()
			wg.Add(1)
			go func() {
				defer func() {
					wasp.L(ctx).Info("publish processor stopped")
					wg.Done()
				}()
				messageLog.Consume(ctx, func(p *packet.Publish) {
					err := wasp.ProcessPublish(ctx, id, rpcTransport, stateMachine, state, true, p)
					if err != nil {
						wasp.L(ctx).Info("publish processing failed", zap.Error(err))
					}
				})
			}()
			remotePublishCh := make(chan *packet.Publish, 20)
			wg.Add(1)
			go func() {
				defer func() {
					wasp.L(ctx).Info("remote publish processor stopped")
					wg.Done()
				}()
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
			}()
			mqttServer := rpc.NewMQTTServer(remotePublishCh)
			mqttServer.Serve(server)
			wg.Add(1)
			go func() {
				defer func() {
					wasp.L(ctx).Info("publish storer stopped")
					wg.Done()
				}()
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
			}()
			wg.Add(1)
			go func() {
				defer func() {
					wasp.L(ctx).Info("cluster listener stopped")
					wg.Done()
				}()
				err := server.Serve(clusterListener)
				if err != nil {
					wasp.L(ctx).Fatal("cluster listener crashed", zap.Error(err))
				}
			}()
			handler := func(m transport.Metadata) error {
				go func() {
					ctx, cancel := context.WithCancel(ctx)
					defer cancel()
					ctx = wasp.AddFields(ctx, zap.String("transport", m.Name), zap.String("remote_address", m.RemoteAddress))
					wasp.RunSession(ctx, stateMachine, state, m.Channel, publishes)
				}()
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
				wasp.L(ctx).Error("failed to leave raft cluster", zap.Error(err))
			}
			err = raftNode.Shutdown(ctx)
			if err != nil {
				wasp.L(ctx).Error("failed to shutdown raft", zap.Error(err))
			} else {
				wasp.L(ctx).Info("raft node stopped")
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
		},
	}

	defaultIP := localPrivateHost()

	cmd.Flags().Bool("debug", false, "Use a fancy logger and increase logging level.")
	cmd.Flags().Bool("use-vault", false, "Use Hashicorp Vault to store private keys and certificates.")
	cmd.Flags().Bool("consul-join", false, "Use Hashicorp Consul to find other gossip members. Wasp won't handle service registration in Consul, you must do it before running Wasp.")
	cmd.Flags().String("consul-service-name", "wasp", "Consul auto-join service name.")
	cmd.Flags().String("consul-service-tag", "gossip", "Consul auto-join service tag.")

	cmd.Flags().Int("metrics-port", 0, "Start Prometheus HTTP metrics server on this port.")
	cmd.Flags().IntP("tcp-port", "t", 0, "Start TCP listener on this port.")
	cmd.Flags().IntP("tls-port", "s", 0, "Start TLS listener on this port.")
	cmd.Flags().IntP("wss-port", "w", 0, "Start Secure WS listener on this port.")
	cmd.Flags().Int("ws-port", 0, "Start WS listener on this port.")
	cmd.Flags().Int("serf-port", 1799, "Membership (Serf) port.")
	cmd.Flags().Int("raft-port", 1899, "Clustering (Raft) port.")
	cmd.Flags().String("serf-advertized-address", defaultIP, "Advertize this adress to other gossip members.")
	cmd.Flags().String("raft-advertized-address", defaultIP, "Advertize this adress to other raft nodes.")
	cmd.Flags().Int("serf-advertized-port", 1799, "Advertize this port to other gossip members.")
	cmd.Flags().Int("raft-advertized-port", 1899, "Advertize this port to other raft nodes.")
	cmd.Flags().StringSliceP("join-node", "j", nil, "Join theses nodes to form a cluster.")
	cmd.Flags().StringP("data-dir", "d", "/tmp/wasp", "Wasp persistent message log location.")

	cmd.Flags().String("tls-cn", "localhost", "Get ACME certificat for this Common Name.")
	cmd.Flags().IntP("raft-bootstrap-expect", "n", 3, "Wasp will wait for this number of nodes to be available before bootstraping a cluster.")

	cmd.Flags().String("rpc-tls-certificate-authority-file", "", "x509 certificate authority used by RPC Server.")
	cmd.Flags().String("rpc-tls-certificate-file", "", "x509 certificate used by RPC Server.")
	cmd.Flags().String("rpc-tls-private-key-file", "", "Private key used by RPC Server.")
	cmd.AddCommand(TLSHelper(config))
	cmd.Execute()
}
