package main

import (
	"context"
	crypto_rand "crypto/rand"
	"encoding/binary"
	"fmt"
	"log"
	"math/rand"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/vx-labs/wasp/cluster/clusterpb"
	"go.uber.org/zap"
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
	raft := &cobra.Command{
		Use: "raft",
	}
	node := &cobra.Command{
		Use: "node",
	}
	mqtt := Mqtt(ctx, config)
	raft.AddCommand(&cobra.Command{
		Use: "members",
		Run: func(cmd *cobra.Command, _ []string) {
			conn, l := mustDial(ctx, cmd, config)
			out, err := clusterpb.NewMultiRaftClient(conn).GetMembers(ctx, &clusterpb.GetMembersRequest{ClusterID: "wasp"})
			if err != nil {
				l.Fatal("failed to list raft members", zap.Error(err))
			}
			table := getTable([]string{"ID", "Leader", "Address", "Health"}, cmd.OutOrStdout())
			for _, member := range out.GetMembers() {
				healthString := "healthy"
				if !member.IsAlive {
					healthString = "unhealthy"
				}
				table.Append([]string{
					fmt.Sprintf("%x", member.GetID()), fmt.Sprintf("%v", member.GetIsLeader()), member.GetAddress(), healthString,
				})
			}
			table.Render()
		},
	})
	raft.AddCommand(&cobra.Command{
		Use: "topology",
		Run: func(cmd *cobra.Command, _ []string) {
			conn, l := mustDial(ctx, cmd, config)
			out, err := clusterpb.NewMultiRaftClient(conn).GetTopology(ctx, &clusterpb.GetTopologyRequest{ClusterID: "wasp"})
			if err != nil {
				l.Fatal("failed to get raft topology", zap.Error(err))
			}
			table := getTable([]string{"ID", "Leader", "Address", "Healthchecks", "Suffrage", "Progress"}, cmd.OutOrStdout())
			for _, member := range out.GetMembers() {
				healthString := "passing"
				suffrageString := "unknown"
				if !member.IsAlive {
					healthString = "error"
				} else {
					if member.IsVoter {
						suffrageString = "voter"
					} else {
						suffrageString = "learner"
					}
				}
				table.Append([]string{
					fmt.Sprintf("%x", member.GetID()),
					fmt.Sprintf("%v", member.GetIsLeader()),
					member.GetAddress(),
					healthString,
					suffrageString,
					fmt.Sprintf("%d/%d", member.GetApplied(), out.Committed),
				})
			}
			table.Render()
		},
	})

	removeMember := &cobra.Command{
		Use: "remove-member",
		Run: func(cmd *cobra.Command, args []string) {
			conn, l := mustDial(ctx, cmd, config)
			id, err := strconv.ParseUint(args[0], 16, 64)
			if err != nil {
				l.Fatal("invalid node id specified", zap.Error(err))
			}
			_, err = clusterpb.NewMultiRaftClient(conn).RemoveMember(ctx, &clusterpb.RemoveMultiRaftMemberRequest{
				ClusterID: "wasp",
				ID:        id,
			})
			if err != nil {
				l.Fatal("failed to remove raft member", zap.Error(err))
			}
		},
	}
	removeMember.Args = cobra.ExactArgs(1)
	raft.AddCommand(removeMember)

	hostname, _ := os.Hostname()

	rootCmd.AddCommand(raft)
	rootCmd.AddCommand(node)
	rootCmd.AddCommand(mqtt)
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
