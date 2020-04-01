package main

import (
	"context"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/vx-labs/wasp/wasp/rpc"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

func mustDial(ctx context.Context, cmd *cobra.Command, config *viper.Viper) (*grpc.ClientConn, *zap.Logger) {
	l := logger(config.GetBool("debug"))
	err := os.MkdirAll(dataDir(), 0750)
	if err != nil {
		l.Fatal("failed to create data directory", zap.Error(err))
	}
	if config.GetBool("use-vault") {
		if !cmd.Flag("rpc-tls-certificate-authority-file").Changed {
			config.Set("rpc-tls-certificate-authority-file", caPath())
		}
		if !cmd.Flag("rpc-tls-certificate-file").Changed {
			config.Set("rpc-tls-certificate-file", certPath())
		}
		if !cmd.Flag("rpc-tls-private-key-file").Changed {
			config.Set("rpc-tls-private-key-file", privkeyPath())
		}
		if areVaultTLSCredentialsExpired() {
			l.Info("refreshing certificates from Vault")
			err = saveVaultCertificate(config.GetString("vault-pki-path"), config.GetString("vault-pki-common-name"))
			if err != nil {
				l.Fatal("failed to fetch GRPC TLS certificates from vault", zap.Error(err))
			}
		}
	}

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
	l.Debug("dialing wasp server", zap.String("remote_host", host))
	conn, err := dialer(host, grpc.WithTimeout(3000*time.Millisecond))
	if err != nil {
		l.Fatal("failed to dial Wasp server", zap.Error(err))
	}
	return conn, l
}
