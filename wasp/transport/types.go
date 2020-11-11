package transport

import (
	"crypto/tls"
	"io"
	"time"
)

type TimeoutReadWriteCloser interface {
	SetDeadline(time.Time) error
	SetReadDeadline(time.Time) error
	SetWriteDeadline(time.Time) error
	io.ReadWriteCloser
}

type Metadata struct {
	Name            string
	Encrypted       bool
	EncryptionState *tls.ConnectionState
	RemoteAddress   string
	Channel         TimeoutReadWriteCloser
	FD              int
}
