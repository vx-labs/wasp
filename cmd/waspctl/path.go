package main

import (
	"os"
	"path"
)

func configDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		panic("failed to find user data dir: " + err.Error())
	}
	return path.Join(home, ".config", "waspctl")
}
func dataDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		panic("failed to find user data dir: " + err.Error())
	}
	return path.Join(home, ".local", "share", "waspctl")
}

func tlsPath() string {
	return path.Join(dataDir(), "tls")
}
func certPath() string {
	return path.Join(tlsPath(), "certificate.pem")
}
func caPath() string {
	return path.Join(tlsPath(), "certificate_authority.pem")
}
func privkeyPath() string {
	return path.Join(tlsPath(), "private_key.pem")
}
