package main

import (
	"context"
	"crypto/tls"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/spf13/viper"
	"go.etcd.io/etcd/raft/raftpb"
	"go.uber.org/zap"

	"github.com/spf13/cobra"
	"github.com/vx-labs/mqtt-protocol/packet"
	"github.com/vx-labs/wasp/vaultacme"
	"github.com/vx-labs/wasp/wasp"
	"github.com/vx-labs/wasp/wasp/fsm"
	"github.com/vx-labs/wasp/wasp/raft"
	"github.com/vx-labs/wasp/wasp/transport"
)

type listenerConfig struct {
	name     string
	port     int
	listener net.Listener
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
			config.BindPFlag("tls-cn", cmd.Flags().Lookup("tls-cn"))
			config.BindPFlag("data-dir", cmd.Flags().Lookup("data-dir"))
			config.BindPFlag("debug", cmd.Flags().Lookup("debug"))
			config.BindPFlag("use-vault", cmd.Flags().Lookup("use-vault"))
		},
		Run: func(cmd *cobra.Command, _ []string) {
			ctx, cancel := context.WithCancel(context.Background())
			ctx = wasp.StoreLogger(ctx, getLogger(config))
			wg := sync.WaitGroup{}
			publishes := make(chan *packet.Publish, 20)
			commandsCh := make(chan raft.Command)
			confCh := make(chan raftpb.ConfChange)
			state := wasp.NewState()
			eventsCh, errCh, snapCh := raft.NewNode(1, config.GetString("data-dir"), []string{"http://127.0.0.1:1899"}, false, state.MarshalBinary, commandsCh, confCh)
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
			stateMachine := fsm.NewFSM("1", state, commandsCh)

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
	cmd.Flags().BoolP("debug", "", false, "Use a fancy logger and increase logging level.")
	cmd.Flags().BoolP("use-vault", "", false, "Use Hashicorp Vault to store private keys and certificates.")

	cmd.Flags().IntP("tcp-port", "t", 0, "Start TCP listener on this port.")
	cmd.Flags().IntP("tls-port", "s", 0, "Start TLS listener on this port.")
	cmd.Flags().IntP("wss-port", "w", 0, "Start Secure WS listener on this port.")
	cmd.Flags().IntP("ws-port", "", 0, "Start WS listener on this port.")

	cmd.Flags().StringP("data-dir", "d", "/tmp/wasp", "Wasp persistent message log location.")

	cmd.Flags().StringP("tls-cn", "", "localhost", "Get ACME certificat for this Common Name.")
	cmd.Execute()
}
