package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/hashicorp/memberlist"
	"github.com/spf13/viper"
	"go.etcd.io/etcd/raft/raftpb"
	"go.uber.org/zap"

	"github.com/spf13/cobra"
	"github.com/vx-labs/mqtt-protocol/packet"
	"github.com/vx-labs/wasp/vaultacme"
	"github.com/vx-labs/wasp/wasp"
	"github.com/vx-labs/wasp/wasp/fsm"
	"github.com/vx-labs/wasp/wasp/membership"
	"github.com/vx-labs/wasp/wasp/raft"
	"github.com/vx-labs/wasp/wasp/transport"
)

type listenerConfig struct {
	name     string
	port     int
	listener net.Listener
}

type MemberlistMemberProvider interface {
	Members() []*memberlist.Node
}

type MemberMetadata struct {
	RaftAddress string `json:"raft_address"`
	ID          string `json:"id"`
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
func encodeMD(id, raftAddress string) []byte {
	md := MemberMetadata{
		ID:          id,
		RaftAddress: raftAddress,
	}
	p, _ := json.Marshal(md)
	return p
}

func waitForNodes(ctx context.Context, mesh MemberlistMemberProvider, expectedNumber int) ([]raft.Peer, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	for {
		nodes := mesh.Members()
		// TODO: look for bootstrapped cluster
		if len(nodes) >= expectedNumber {
			peers := make([]raft.Peer, len(nodes))
			for idx := range peers {
				md, err := decodeMD(nodes[idx].Meta)
				if err != nil {
					log.Print(string(nodes[idx].Meta))
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

func main() {
	config := viper.New()
	cmd := &cobra.Command{
		Use: "wasp",
		PreRun: func(cmd *cobra.Command, _ []string) {
			config.BindPFlag("tcp-port", cmd.Flags().Lookup("tcp-port"))
			config.BindPFlag("tls-port", cmd.Flags().Lookup("tls-port"))
			config.BindPFlag("wss-port", cmd.Flags().Lookup("wss-port"))
			config.BindPFlag("ws-port", cmd.Flags().Lookup("ws-port"))
			config.BindPFlag("raft-port", cmd.Flags().Lookup("raft-port"))
			config.BindPFlag("serf-port", cmd.Flags().Lookup("serf-port"))
			config.BindPFlag("tls-cn", cmd.Flags().Lookup("tls-cn"))
			config.BindPFlag("data-dir", cmd.Flags().Lookup("data-dir"))
			config.BindPFlag("debug", cmd.Flags().Lookup("debug"))
			config.BindPFlag("use-vault", cmd.Flags().Lookup("use-vault"))
			config.BindPFlag("join-node", cmd.Flags().Lookup("join-node"))
			config.BindPFlag("serf-advertized-address", cmd.Flags().Lookup("serf-advertized-address"))
			config.BindPFlag("raft-advertized-address", cmd.Flags().Lookup("raft-advertized-address"))
			config.BindPFlag("serf-advertized-port", cmd.Flags().Lookup("serf-advertized-port"))
			config.BindPFlag("raft-advertized-port", cmd.Flags().Lookup("raft-advertized-port"))
			config.BindPFlag("raft-bootstrap-expect", cmd.Flags().Lookup("raft-bootstrap-expect"))

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

			id, err := loadID(config.GetString("data-dir"))
			if err != nil {
				wasp.L(ctx).Fatal("failed to get node ID", zap.Error(err))
			}
			ctx = wasp.AddFields(ctx, zap.String("node_id", id))

			wg := sync.WaitGroup{}
			publishes := make(chan *packet.Publish, 20)
			commandsCh := make(chan raft.Command)
			confCh := make(chan raftpb.ConfChange)
			state := wasp.NewState()

			membership := membership.New(
				id,
				config.GetInt("serf-port"),
				config.GetString("serf-advertized-address"),
				config.GetInt("serf-advertized-port"),
				wasp.L(ctx),
			)
			fmt.Printf("Gossip listener is running on port %d", config.GetInt("serf-port"))
			membership.UpdateMetadata(encodeMD(id,
				fmt.Sprintf("http://%s:%d", config.GetString("raft-advertized-address"), config.GetInt("raft-port")),
			))

			joinList := config.GetStringSlice("join-node")
			if len(joinList) > 0 {
				retryTicker := time.NewTicker(3 * time.Second)
				for {
					err = membership.Join(config.GetStringSlice("join-node"))
					if err != nil {
						wasp.L(ctx).Warn("failed to join cluster", zap.Error(err))
					} else {
						break
					}
					<-retryTicker.C
				}
				retryTicker.Stop()
			}
			var peers []raft.Peer
			if n := config.GetInt("raft-bootstrap-expect"); n > 1 {
				wasp.L(ctx).Debug("waiting for nodes to be discovered", zap.Int("expected_node_count", n))
				peers, err = waitForNodes(ctx, membership, n)
				if err != nil {
					wasp.L(ctx).Fatal("cluster bootstrap failed", zap.Error(err))
				}
				wasp.L(ctx).Debug("found nodes")
			}
			wasp.L(ctx).Debug("starting raft")
			eventsCh, errCh, snapCh := raft.NewNode(raft.Config{
				NodeID:      id,
				Peers:       peers,
				NodeAddress: fmt.Sprintf("0.0.0.0:%d", config.GetInt("raft-port")),
				DataDir:     config.GetString("data-dir"),
				Join:        false,
				GetSnapshot: state.MarshalBinary,
				ProposeC:    commandsCh,
				ConfChangeC: confCh,
			})
			snapshotter := <-snapCh

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
					wasp.L(ctx).Info("raft error processor stopped")
					wg.Done()
				}()
				for {
					select {
					case <-ctx.Done():
						return
					case err := <-errCh:
						wasp.L(ctx).Fatal("raft error", zap.Error(err))
					}
				}
			}()

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
					case event := <-eventsCh:
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
					err := wasp.ProcessPublish(state, p)
					if err != nil {
						wasp.L(ctx).Info("publish processing failed", zap.Error(err))
					}
				})
			}()
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
				wasp.L(ctx).Info("listener started", zap.String("listener_name", listener.name), zap.Int("listener_port", listener.port))
			}
			sigc := make(chan os.Signal, 1)
			signal.Notify(sigc,
				syscall.SIGINT,
				syscall.SIGTERM,
				syscall.SIGQUIT)
			<-sigc
			wasp.L(ctx).Info("shutting down")
			for _, listener := range listeners {
				listener.listener.Close()
				wasp.L(ctx).Info("listener stopped", zap.String("listener_name", listener.name), zap.Int("listener_port", listener.port))
			}
			cancel()
			wg.Wait()
			err = messageLog.Close()
			if err != nil {
				wasp.L(ctx).Error("failed to close message log", zap.Error(err))
			} else {
				wasp.L(ctx).Info("message log closed")
			}
		},
	}

	defaultIP := localPrivateHost()

	cmd.Flags().Bool("debug", false, "Use a fancy logger and increase logging level.")
	cmd.Flags().Bool("use-vault", false, "Use Hashicorp Vault to store private keys and certificates.")

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
	cmd.Execute()
}
