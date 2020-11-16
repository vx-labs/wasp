package main

import (
	"context"
	crypto_rand "crypto/rand"
	"encoding/binary"
	"log"
	"math/rand"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func seedRand() {
	var b [8]byte
	_, err := crypto_rand.Read(b[:])
	if err != nil {
		panic("cannot seed math/rand package with cryptographically secure random number generator")
	}
	rand.Seed(int64(binary.LittleEndian.Uint64(b[:])))
}
func main() {
	seedRand()
	config := viper.New()
	config.AddConfigPath(configDir())
	config.SetConfigType("yaml")
	config.SetConfigName("config")
	config.SetEnvPrefix("WASPCTL")
	config.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	config.AutomaticEnv()
	ctx := context.Background()
	rootCmd := &cobra.Command{
		PersistentPreRun: func(cmd *cobra.Command, _ []string) {
			config.BindPFlags(cmd.Flags())
			config.BindPFlags(cmd.PersistentFlags())
			if err := config.ReadInConfig(); err != nil {
				if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
					log.Fatal(err)
				}
			}
		},
	}

	hostname, _ := os.Hostname()
	rootCmd.AddCommand(Sessions(ctx, config))
	rootCmd.AddCommand(Subscriptions(ctx, config))
	rootCmd.AddCommand(Messages(ctx, config))
	rootCmd.AddCommand(Topics(ctx, config))
	rootCmd.AddCommand(Cluster(ctx, config))

	rootCmd.PersistentFlags().BoolP("insecure", "k", false, "Disable GRPC client-side TLS validation.")
	rootCmd.PersistentFlags().BoolP("debug", "d", false, "Increase log verbosity.")
	rootCmd.PersistentFlags().BoolP("use-vault", "v", false, "Use Hashicorp Vault to generate GRPC Certificates.")
	rootCmd.PersistentFlags().String("vault-pki-path", "pki/issue/grpc", "Vault PKI certificate issuing path.")
	rootCmd.PersistentFlags().String("vault-pki-common-name", hostname, "Vault PKI certificate Common Name to submit.")
	rootCmd.PersistentFlags().BoolP("use-consul", "c", false, "Use Hashicorp Consul to find Wasp server.")
	rootCmd.PersistentFlags().String("consul-service-name", "wasp", "Consul service name.")
	rootCmd.PersistentFlags().String("consul-service-tag", "rpc", "Consul service tag.")
	rootCmd.PersistentFlags().String("host", "127.0.0.1:1899", "remote GRPC endpoint")
	rootCmd.PersistentFlags().String("rpc-tls-certificate-authority-file", "", "x509 certificate authority used by RPC Client.")
	rootCmd.PersistentFlags().String("rpc-tls-certificate-file", "", "x509 certificate used by RPC Client.")
	rootCmd.PersistentFlags().String("rpc-tls-private-key-file", "", "Private key used by RPC Client.")
	rootCmd.Execute()
}
