package main

import (
	"context"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/vx-labs/wasp/wasp/rpc"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

func mustDial(ctx context.Context, cmd *cobra.Command, config *viper.Viper) (*grpc.ClientConn, *zap.Logger) {
	l := logger(config.GetBool("debug"))
	dialer := rpc.GRPCDialer(rpc.ClientConfig{
		InsecureSkipVerify:          config.GetBool("insecure"),
		TLSCertificateAuthorityPath: config.GetString("rpc-tls-certificate-authority-file"),
		TLSCertificatePath:          config.GetString("rpc-tls-certificate-file"),
		TLSPrivateKeyPath:           config.GetString("rpc-tls-private-key-file"),
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
