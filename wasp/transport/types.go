package transport

import (
	"context"
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

type ConnectionSetuper interface {
	Setup(ctx context.Context, c Metadata)
}

type Metadata struct {
	Name            string
	Encrypted       bool
	EncryptionState *tls.ConnectionState
	RemoteAddress   string
	Channel         TimeoutReadWriteCloser
}
