package main

import (
	"context"
	"fmt"
	"io"
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

func logger() *zap.Logger {
	config := zap.NewProductionEncoderConfig()
	config.EncodeLevel = zapcore.CapitalColorLevelEncoder
	return zap.New(zapcore.NewCore(
		zapcore.NewConsoleEncoder(config),
		zapcore.AddSync(colorable.NewColorableStdout()),
		zapcore.InfoLevel,
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

func findServer(service, tag string) (string, error) {
	client, err := consul.NewClient(consul.DefaultConfig())
	if err != nil {
		return "", err
	}
	services, _, err := client.Catalog().Service(service, tag, &consul.QueryOptions{})
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s:%d", services[0].ServiceAddress, services[0].ServicePort), nil
}

func main() {
	config := viper.New()
	ctx := context.Background()
	l := logger()
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
	raft.AddCommand(&cobra.Command{
		Use: "members",
		Run: func(cmd *cobra.Command, _ []string) {
			dialer := rpc.GRPCDialer(rpc.ClientConfig{
				TLSCertificateAuthorityPath: config.GetString("rpc-tls-certificate-authority-file"),
			})
			host := config.GetString("host")
			if cmd.Flags().Changed("host") && config.GetBool("use-consul") {
				var err error
				host, err = findServer(config.GetString("consul-service-name"), config.GetString("consul-service-tag"))
				if err != nil {
					l.Fatal("failed to find Wasp server using Consul", zap.Error(err))
				}
			}
			conn, err := dialer(host, grpc.WithBlock(), grpc.WithTimeout(3000*time.Millisecond))
			if err != nil {
				l.Fatal("failed to dial Wasp server", zap.Error(err), zap.String("remote_host", host))
			}
			l.Debug("connected to Wasp server", zap.String("remote_host", host))
			out, err := api.NewRaftClient(conn).GetMembers(ctx, &api.GetMembersRequest{})
			if err != nil {
				l.Fatal("failed to list raft members", zap.Error(err))
			}
			l.Debug("listed raft members", zap.String("remote_host", host))
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
			dialer := rpc.GRPCDialer(rpc.ClientConfig{
				TLSCertificateAuthorityPath: config.GetString("rpc-tls-certificate-authority-file"),
			})
			host := config.GetString("host")
			if host == "" && config.GetBool("use-consul") {
				var err error
				host, err = findServer(config.GetString("consul-service-name"), config.GetString("consul-service-tag"))
				if err != nil {
					l.Fatal("failed to find Wasp server using Consul", zap.Error(err))
				}
			}
			conn, err := dialer(host, grpc.WithBlock(), grpc.WithTimeout(3000*time.Millisecond))
			if err != nil {
				l.Fatal("failed to dial Wasp server", zap.Error(err), zap.String("remote_host", host))
			}
			l.Debug("connected to Wasp server", zap.String("remote_host", host))
			_, err = api.NewNodeClient(conn).Shutdown(ctx, &api.ShutdownRequest{})
			if err != nil {
				l.Fatal("failed to shutdown node", zap.Error(err))
			}
			fmt.Println("Shutdown started")
		},
	})
	rootCmd.AddCommand(raft)
	rootCmd.AddCommand(node)
	rootCmd.PersistentFlags().BoolP("use-consul", "c", false, "Use Hashicorp Consul to find Wasp server.")
	rootCmd.PersistentFlags().String("consul-service-name", "wasp", "Consul service name.")
	rootCmd.PersistentFlags().String("consul-service-tag", "rpc", "Consul service tag.")
	rootCmd.PersistentFlags().String("host", "127.0.0.1:1899", "remote GRPC endpoint")
	rootCmd.PersistentFlags().String("rpc-tls-certificate-authority-file", "", "x509 certificate authority used by RPC Client.")

	rootCmd.Execute()
}
