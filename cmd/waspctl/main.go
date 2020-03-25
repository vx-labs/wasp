package main

import (
	"context"
	"fmt"
	"io"
	"time"

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
		opts = append(opts, tablewriter.Colors{tablewriter.Bold})
	}
	table.SetHeaderColor(opts...)
	return table
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
	raft.AddCommand(&cobra.Command{
		Use: "members",
		Run: func(cmd *cobra.Command, _ []string) {
			dialer := rpc.GRPCDialer(rpc.ClientConfig{
				TLSCertificateAuthorityPath: config.GetString("rpc-tls-certificate-authority-file"),
			})
			host := config.GetString("host")
			conn, err := dialer(host, grpc.WithBlock(), grpc.WithTimeout(500*time.Millisecond), grpc.WithDisableRetry())
			if err != nil {
				l.Fatal("failed to dial Wasp server", zap.Error(err), zap.String("remote_host", host))
			}
			l.Debug("connected to Wasp server", zap.String("remote_host", host))
			out, err := api.NewRaftClient(conn).GetMembers(ctx, &api.GetMembersRequest{})
			if err != nil {
				l.Fatal("failed to list raft members", zap.Error(err))
			}
			l.Debug("listed raft members", zap.String("remote_host", host))
			table := getTable([]string{"ID", "Leader ?", "Address"}, cmd.OutOrStdout())
			for _, member := range out.GetMembers() {
				table.Append([]string{
					fmt.Sprintf("%x", member.GetID()), fmt.Sprintf("%v", member.GetIsLeader()), member.GetAddress(),
				})
			}
			table.Render()
		},
	})
	rootCmd.AddCommand(raft)
	rootCmd.PersistentFlags().String("host", "", "remote GRPC endpoint")
	rootCmd.PersistentFlags().String("rpc-tls-certificate-authority-file", "", "x509 certificate authority used by RPC Client.")

	rootCmd.Execute()
}
