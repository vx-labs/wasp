package rpc

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"errors"
	"fmt"
	"io/ioutil"
	"math/big"
	"net"
	"os"
	"time"

	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

func init() {
	grpc_prometheus.EnableHandlingTimeHistogram()
}

type ServerConfig struct {
	TLSCertificateAuthorityPath string
	TLSCertificatePath          string
	TLSPrivateKeyPath           string
	VerifyClientCert            bool
}
type ClientConfig struct {
	TLSCertificateAuthorityPath string
	TLSCertificatePath          string
	TLSPrivateKeyPath           string
	InsecureSkipVerify          bool
}

func loadTLSConfig(caPath, crtPath, keyPath string) *tls.Config {
	certificate, err := tls.LoadX509KeyPair(crtPath, keyPath)
	if err != nil {
		panic(fmt.Errorf("could not load server key pair: %s", err))
	}

	// Create a certificate pool from the certificate authority
	var certPool *x509.CertPool
	if caPath != "" {
		certPool = x509.NewCertPool()
		ca, err := ioutil.ReadFile(caPath)
		if err != nil {
			panic(fmt.Errorf("could not read ca certificate: %s", err))
		}

		// Append the client certificates from the CA
		if ok := certPool.AppendCertsFromPEM(ca); !ok {
			panic(errors.New("failed to append client certs to certificate pool"))
		}
	} else {
		certPool, err = x509.SystemCertPool()
		if err != nil {
			panic(err)
		}
	}
	return &tls.Config{
		ClientAuth:   tls.RequireAndVerifyClientCert,
		Certificates: []tls.Certificate{certificate},
		ClientCAs:    certPool,
		RootCAs:      certPool,
	}
}

func GenerateSelfSignedCertificate(cn string, san []string, ipAddresses []net.IP) (*tls.Certificate, error) {
	privkey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, err
	}
	certTemplate := &x509.Certificate{
		NotAfter:     time.Now().Add(12 * 30 * 24 * time.Hour),
		SerialNumber: big.NewInt(1),
		IPAddresses:  ipAddresses,
		DNSNames:     san,
		Subject: pkix.Name{
			CommonName: cn,
		},
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:    x509.KeyUsageDigitalSignature,
	}

	certBody, err := x509.CreateCertificate(rand.Reader, certTemplate, certTemplate, privkey.Public(), privkey)
	if err != nil {
		return nil, err
	}
	cert, err := x509.ParseCertificate(certBody)
	if err != nil {
		return nil, err
	}
	return &tls.Certificate{
		Certificate: [][]byte{certBody},
		Leaf:        cert,
		PrivateKey:  privkey,
	}, nil
}

func ListLocalIP() []net.IP {
	out := []net.IP{}
	ifaces, err := net.Interfaces()
	if err != nil {
		panic(err)
	}
	for _, i := range ifaces {
		addrs, err := i.Addrs()
		if err != nil {
			panic(err)
		}
		for _, addr := range addrs {
			switch v := addr.(type) {
			case *net.IPNet:
				out = append(out, v.IP)
			case *net.IPAddr:
				out = append(out, v.IP)
			}
		}
	}
	return out
}

func Server(config ServerConfig) *grpc.Server {
	return grpc.NewServer(GRPCServerOptions(config.VerifyClientCert, config.TLSCertificateAuthorityPath, config.TLSCertificatePath, config.TLSPrivateKeyPath)...)
}

func GRPCServerOptions(mTLS bool, tlsCertificateAuthorityPath, tlsCertificatePath, tlsPrivateKey string) []grpc.ServerOption {
	opts := []grpc.ServerOption{grpc.StreamInterceptor(grpc_prometheus.StreamServerInterceptor),
		grpc.UnaryInterceptor(grpc_prometheus.UnaryServerInterceptor),
	}
	var tlsCreds credentials.TransportCredentials
	var err error
	if mTLS {
		tlsConfig := loadTLSConfig(tlsCertificateAuthorityPath, tlsCertificatePath, tlsPrivateKey)
		tlsCreds = credentials.NewTLS(tlsConfig)
	} else {
		if tlsCertificatePath != "" && tlsPrivateKey != "" {
			tlsCreds, err = credentials.NewServerTLSFromFile(tlsCertificatePath, tlsPrivateKey)
			if err != nil {
				panic(err)
			}
		} else {
			tlsCertificate, err := GenerateSelfSignedCertificate(os.Getenv("HOSTNAME"), []string{"*"}, append(ListLocalIP()))
			if err != nil {
				panic(err)
			}
			tlsCreds = credentials.NewServerTLSFromCert(tlsCertificate)
		}
	}
	return append(opts,
		grpc.Creds(tlsCreds),
	)
}

type Dialer func(address string, opts ...grpc.DialOption) (*grpc.ClientConn, error)

func GRPCDialer(config ClientConfig) Dialer {
	return func(address string, opts ...grpc.DialOption) (*grpc.ClientConn, error) {
		return grpc.Dial(address, append(opts,
			GRPCClientOptions(
				config.TLSCertificateAuthorityPath,
				config.TLSCertificatePath,
				config.TLSPrivateKeyPath,
				config.InsecureSkipVerify,
			)...)...)
	}
}

func GRPCClientOptions(tlsCertificateAuthorityPath, tlsCertificatePath, tlsPrivateKey string, insecureSkipVerify bool) []grpc.DialOption {
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
	if tlsCertificatePath != "" && tlsPrivateKey != "" && tlsCertificateAuthorityPath != "" {
		tlsConfig := loadTLSConfig(tlsCertificateAuthorityPath, tlsCertificatePath, tlsPrivateKey)
		tlsConfig.InsecureSkipVerify = insecureSkipVerify
		return append(dialOpts, grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)))
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
