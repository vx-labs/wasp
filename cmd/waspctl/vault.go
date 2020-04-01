package main

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"io"
	"os"
	"time"

	vault "github.com/hashicorp/vault/api"
)

func areVaultTLSCredentialsExpired() bool {
	for _, filename := range []string{caPath(), certPath(), privkeyPath()} {
		if _, err := os.Stat(filename); err != nil {
			return true
		}
	}
	certificates, err := tls.LoadX509KeyPair(certPath(), privkeyPath())
	if err != nil {
		return true
	}
	if certificates.Certificate[0] == nil {
		return true
	}
	cert, err := x509.ParseCertificate(certificates.Certificate[0])
	if err != nil {
		return true
	}
	return time.Now().After(cert.NotAfter)
}

func saveVaultCertificate(pkiPath, cn string) error {
	err := os.MkdirAll(tlsPath(), 0700)
	if err != nil {
		return err
	}
	client, err := vault.NewClient(vault.DefaultConfig())
	if err != nil {
		return err
	}
	out, err := client.Logical().Write(pkiPath, map[string]interface{}{
		"common_name": cn,
		"ttl":         "48h",
	})
	if err != nil {
		return err
	}
	err = copyBufToFile(certPath(),
		bytes.NewBufferString(out.Data["certificate"].(string)))
	if err != nil {
		return err
	}
	err = copyBufToFile(privkeyPath(),
		bytes.NewBufferString(out.Data["private_key"].(string)))
	if err != nil {
		return err
	}
	err = copyBufToFile(caPath(),
		bytes.NewBufferString(out.Data["issuing_ca"].(string)))
	if err != nil {
		return err
	}
	return nil
}

func copyBufToFile(p string, r io.Reader) error {
	to, err := os.Create(p)
	if err != nil {
		return err
	}
	defer to.Close()
	_, err = io.Copy(to, r)
	return err
}
