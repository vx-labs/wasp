package main

import (
	"context"
	crypto_rand "crypto/rand"
	"encoding/binary"
	"fmt"
	"io"
	"math/rand"
	"time"

	consul "github.com/hashicorp/consul/api"

	"github.com/mattn/go-colorable"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/vx-labs/wasp/wasp/api"
	"github.com/vx-labs/wasp/wasp/rpc"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"google.golang.org/grpc"
)

func logger(enableDebug bool) *zap.Logger {
	config := zap.NewProductionEncoderConfig()
	config.EncodeLevel = zapcore.CapitalColorLevelEncoder
	config.EncodeTime = func(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
		enc.AppendString(t.Format(time.Kitchen))
	}
	level := zapcore.InfoLevel
	if enableDebug {
		level = zapcore.DebugLevel
	}
	return zap.New(zapcore.NewCore(
		zapcore.NewConsoleEncoder(config),
		zapcore.AddSync(colorable.NewColorableStderr()),
		level,
	))
}

func getTable(headers []string, out io.Writer) *tablewriter.Table {
	table := tablewriter.NewWriter(out)
	table.SetHeader(headers)
	table.SetAlignment(tablewriter.ALIGN_LEFT)
	table.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
	table.SetCenterSeparator("")
	table.SetColumnSeparator("")
	table.SetRowSeparator("")
	table.SetHeaderLine(false)
	table.SetBorder(false)
	table.SetTablePadding("\t")
	table.SetNoWhiteSpace(true)
	table.SetAutoFormatHeaders(false)

	opts := []tablewriter.Colors{}
	for range headers {
		opts = append(opts, tablewriter.Colors{tablewriter.Bold, tablewriter.FgHiCyanColor})
	}
	table.SetHeaderColor(opts...)
	return table
}

func findServer(l *zap.Logger, service, tag string) (string, error) {
	l.Debug("discovering Wasp server using Consul", zap.String("consul_service_name", service), zap.String("consul_service_tag", tag))
	client, err := consul.NewClient(consul.DefaultConfig())
	if err != nil {
		return "", err
	}
	services, _, err := client.Catalog().Service(service, tag, &consul.QueryOptions{})
	if err != nil {
		return "", err
	}
	var idx int
	if len(services) > 1 {
		l.Debug("discovered multiple Wasp servers using Consul", zap.Int("wasp_server_count", len(services)))
		idx = rand.Intn(len(services))
	} else if len(services) == 1 {
		idx = 0
	} else {
		l.Fatal("no Wasp server discovered using Consul")
	}
	return fmt.Sprintf("%s:%d", services[idx].ServiceAddress, services[idx].ServicePort), nil
}

func mustDial(ctx context.Context, cmd *cobra.Command, config *viper.Viper) (*grpc.ClientConn, *zap.Logger) {
	l := logger(config.GetBool("debug"))
	dialer := rpc.GRPCDialer(rpc.ClientConfig{
		InsecureSkipVerify:          config.GetBool("insecure"),
		TLSCertificateAuthorityPath: config.GetString("rpc-tls-certificate-authority-file"),
	})
	host := config.GetString("host")
	if !cmd.Flags().Changed("host") && config.GetBool("use-consul") {
		var err error
		serviceName := config.GetString("consul-service-name")
		serviceTag := config.GetString("consul-service-tag")
		host, err = findServer(l, serviceName, serviceTag)
		if err != nil {
			l.Fatal("failed to find Wasp server using Consul", zap.Error(err))
		}
	}
	l = l.With(zap.String("remote_host", host))
	l.Debug("using remote host")
	l.Debug("dialing wasp server", zap.String("remote_host", host))
	conn, err := dialer(host, grpc.WithTimeout(3000*time.Millisecond))
	if err != nil {
		l.Fatal("failed to dial Wasp server", zap.Error(err))
	}
	return conn, l
}

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
	ctx := context.Background()
	rootCmd := &cobra.Command{
		PersistentPreRun: func(cmd *cobra.Command, _ []string) {
			config.BindPFlags(cmd.Flags())
			config.BindPFlags(cmd.PersistentFlags())
		},
	}
	raft := &cobra.Command{
		Use: "raft",
	}
	node := &cobra.Command{
		Use: "node",
	}
	mqtt := &cobra.Command{
		Use: "mqtt",
	}
	mqtt.AddCommand(&cobra.Command{
		Use: "list-sessions",
		Run: func(cmd *cobra.Command, _ []string) {
			conn, l := mustDial(ctx, cmd, config)
			out, err := api.NewMQTTClient(conn).ListSessionMetadatas(ctx, &api.ListSessionMetadatasRequest{})
			if err != nil {
				l.Fatal("failed to list connected sessions", zap.Error(err))
			}
			table := getTable([]string{"ID", "Peer", "Connected Since"}, cmd.OutOrStdout())
			for _, member := range out.GetSessionMetadatasList() {
				table.Append([]string{
					member.GetSessionID(),
					fmt.Sprintf("%x", member.GetPeer()),
					time.Since(time.Unix(member.GetConnectedAt(), 0)).String(),
				})
			}
			table.Render()
		},
	})
	raft.AddCommand(&cobra.Command{
		Use: "members",
		Run: func(cmd *cobra.Command, _ []string) {
			conn, l := mustDial(ctx, cmd, config)
			out, err := api.NewRaftClient(conn).GetMembers(ctx, &api.GetMembersRequest{})
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
	node.AddCommand(&cobra.Command{
		Use: "shutdown",
		Run: func(cmd *cobra.Command, args []string) {
			conn, l := mustDial(ctx, cmd, config)
			_, err := api.NewNodeClient(conn).Shutdown(ctx, &api.ShutdownRequest{})
			if err != nil {
				l.Fatal("failed to shutdown node", zap.Error(err))
			}
			fmt.Println("Shutdown started")
		},
	})
	rootCmd.AddCommand(raft)
	rootCmd.AddCommand(node)
	rootCmd.AddCommand(mqtt)
	rootCmd.PersistentFlags().BoolP("insecure", "k", false, "Disable GRPC client-side TLS validation.")
	rootCmd.PersistentFlags().BoolP("debug", "d", false, "Increase log verbosity.")
	rootCmd.PersistentFlags().BoolP("use-consul", "c", false, "Use Hashicorp Consul to find Wasp server.")
	rootCmd.PersistentFlags().String("consul-service-name", "wasp", "Consul service name.")
	rootCmd.PersistentFlags().String("consul-service-tag", "rpc", "Consul service tag.")
	rootCmd.PersistentFlags().String("host", "127.0.0.1:1899", "remote GRPC endpoint")
	rootCmd.PersistentFlags().String("rpc-tls-certificate-authority-file", "", "x509 certificate authority used by RPC Client.")

	rootCmd.Execute()
}
