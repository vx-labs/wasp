package rpc

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"

	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

func init() {
	grpc_prometheus.EnableHandlingTimeHistogram()
}

type ServerConfig struct {
	TLSCertificatePath string
	TLSPrivateKeyPath  string
}
type ClientConfig struct {
	TLSCertificateAuthorityPath string
	InsecureSkipVerify          bool
}

func Server(config ServerConfig) *grpc.Server {
	return grpc.NewServer(GRPCServerOptions(config.TLSCertificatePath, config.TLSPrivateKeyPath)...)
}

func GRPCServerOptions(tlsCertificatePath string, tlsPrivateKey string) []grpc.ServerOption {
	opts := []grpc.ServerOption{grpc.StreamInterceptor(grpc_prometheus.StreamServerInterceptor),
		grpc.UnaryInterceptor(grpc_prometheus.UnaryServerInterceptor),
	}
	if tlsCertificatePath != "" && tlsPrivateKey != "" {
		tlsCreds, err := credentials.NewServerTLSFromFile(tlsCertificatePath, tlsPrivateKey)
		if err != nil {
			panic(err)
		}
		return append(opts,
			grpc.Creds(tlsCreds),
		)
	}
	return opts
}

type Dialer func(address string, opts ...grpc.DialOption) (*grpc.ClientConn, error)

func GRPCDialer(config ClientConfig) Dialer {
	return func(address string, opts ...grpc.DialOption) (*grpc.ClientConn, error) {
		return grpc.Dial(address, append(opts, GRPCClientOptions(config.TLSCertificateAuthorityPath, config.InsecureSkipVerify)...)...)
	}
}

func GRPCClientOptions(tlsCertificateAuthorityPath string, insecureSkipVerify bool) []grpc.DialOption {
	dialOpts := []grpc.DialOption{
		grpc.WithStreamInterceptor(
			grpc_middleware.ChainStreamClient(
				grpc_prometheus.StreamClientInterceptor,
			),
		),
		grpc.WithUnaryInterceptor(
			grpc_middleware.ChainUnaryClient(
				grpc_prometheus.UnaryClientInterceptor,
			),
		),
	}
	if tlsCertificateAuthorityPath != "" {
		tlsConfig, err := credentials.NewClientTLSFromFile(tlsCertificateAuthorityPath, "")
		if err != nil {
			panic(err)
		}
		return append(dialOpts, grpc.WithTransportCredentials(tlsConfig))
	}
	systemCertPool, err := x509.SystemCertPool()
	if err != nil {
		panic(fmt.Sprintf("failed to load system certificate authorities: %v", err))
	}
	tlsConfig := credentials.NewTLS(&tls.Config{
		InsecureSkipVerify: insecureSkipVerify,
		RootCAs:            systemCertPool,
	})
	return append(dialOpts, grpc.WithTransportCredentials(tlsConfig))
}
