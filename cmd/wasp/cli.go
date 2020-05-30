package main

import (
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	consulapi "github.com/hashicorp/consul/api"

	"github.com/spf13/viper"

	"github.com/spf13/cobra"
)

type listenerConfig struct {
	name     string
	port     int
	listener net.Listener
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

func main() {
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
		Run: func(_ *cobra.Command, _ []string) {
			run(config)
		},
	}

	defaultIP := localPrivateHost()

	cmd.Flags().Bool("pprof", false, "Start pprof endpoint.")
	cmd.Flags().Int("pprof-port", 8080, "Profiling (pprof) port.")
	cmd.Flags().String("pprof-address", "127.0.0.1", "Profiling (pprof) port.")
	cmd.Flags().Bool("fancy-logs", false, "Use a fancy logger.")
	cmd.Flags().String("log-level", "error", "Select the loggers- log level")
	cmd.Flags().Bool("use-vault", false, "Use Hashicorp Vault to store private keys and certificates.")
	cmd.Flags().Bool("mtls", false, "Enforce GRPC service-side TLS certificates validation for client connections.")
	cmd.Flags().Bool("insecure", false, "Disable GRPC client-side TLS validation.")
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
	cmd.Flags().IntP("raft-bootstrap-expect", "n", 1, "Wasp will wait for this number of nodes to be available before bootstraping a cluster.")

	cmd.Flags().String("rpc-tls-certificate-authority-file", "", "x509 certificate authority used by RPC Server.")
	cmd.Flags().String("rpc-tls-certificate-file", "", "x509 certificate used by RPC Server.")
	cmd.Flags().String("rpc-tls-private-key-file", "", "Private key used by RPC Server.")

	cmd.Flags().String("syslog-tap-address", "", "Syslog address to send a copy of all message published.")
	cmd.Flags().String("nest-tap-address", "", "Nest address to send a copy of all message published.")

	cmd.Flags().String("authentication-provider", "none", "Authentication mecanism to use to authenticate MQTT clients. Set to \"none\" to disable authentication.")
	cmd.Flags().String("authentication-provider-file-path", "credentials.csv", "Read allowed client credentials from this file, when using \"file\" authentication provider.")
	cmd.Flags().String("authentication-provider-static-username", "", "Client username, when using \"static\" authentication provider.")
	cmd.Flags().String("authentication-provider-static-password", "", "Client password, when using \"static\" authentication provider.")
	cmd.Flags().String("authentication-provider-grpc-address", "", "GRPC Authentication server address, when using \"grpc\" authentication provider.")

	cmd.AddCommand(TLSHelper(config))
	cmd.Execute()
}
