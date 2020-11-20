package main

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"log"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/vx-labs/wasp/v4/rpc"
)

func TLSHelper(config *viper.Viper) *cobra.Command {
	c := &cobra.Command{
		Use:   "tls",
		Short: "Generate TLS certificate and private key.",
		PreRun: func(c *cobra.Command, _ []string) {
			config.BindPFlag("certificate-file", c.Flags().Lookup("certificate-file"))
			config.BindPFlag("private-key-file", c.Flags().Lookup("private-key-file"))
		},
		Run: func(cmd *cobra.Command, _ []string) {
			log.Printf("INFO: generating self-signed TLS certificate.")
			log.Printf("INFO: if this operation seems too long, check this host's entropy.")
			tlsCert, err := rpc.GenerateSelfSignedCertificate(os.Getenv("HOSTNAME"), []string{"*"}, rpc.ListLocalIP())
			if err != nil {
				log.Printf("ERR: %v", err)
				return
			}
			certFile, err := os.Create(config.GetString("certificate-file"))
			if err != nil {
				log.Printf("ERR: %v", err)
				return
			}
			defer certFile.Close()
			keyFile, err := os.Create(config.GetString("private-key-file"))
			if err != nil {
				log.Printf("ERR: %v", err)
				return
			}
			defer keyFile.Close()
			pem.Encode(keyFile, &pem.Block{Type: "PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(tlsCert.PrivateKey.(*rsa.PrivateKey))})
			pem.Encode(certFile, &pem.Block{Type: "CERTIFICATE", Bytes: tlsCert.Leaf.Raw})
		},
	}
	c.Flags().StringP("certificate-file", "c", "./run_config/cert.pem", "Write certificate to this file")
	c.Flags().StringP("private-key-file", "k", "./run_config/privkey.pem", "Write private key to this file")
	return c
}
