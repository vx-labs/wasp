package rpc

import (
	"time"

	grpc_retry "github.com/grpc-ecosystem/go-grpc-middleware/retry"

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
		return grpc.Dial(address, append(opts, GRPCClientOptions(config.TLSCertificateAuthorityPath)...)...)
	}
}

func GRPCClientOptions(tlsCertificateAuthorityPath string) []grpc.DialOption {
	callOpts := []grpc_retry.CallOption{
		grpc_retry.WithBackoff(grpc_retry.BackoffLinearWithJitter(200*time.Millisecond, 0.3)),
		grpc_retry.WithMax(3),
	}
	dialOpts := []grpc.DialOption{
		grpc.WithStreamInterceptor(
			grpc_middleware.ChainStreamClient(
				grpc_prometheus.StreamClientInterceptor,
				grpc_retry.StreamClientInterceptor(callOpts...),
			),
		),
		grpc.WithUnaryInterceptor(
			grpc_middleware.ChainUnaryClient(
				grpc_prometheus.UnaryClientInterceptor,
				grpc_retry.UnaryClientInterceptor(callOpts...),
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
	return append(dialOpts, grpc.WithInsecure())
}
