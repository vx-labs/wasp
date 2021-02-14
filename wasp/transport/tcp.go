package transport

import (
	"context"
	"fmt"
	"net"
)

type tcp struct {
	listener net.Listener
}

func NewTCPTransport(ctx context.Context, port int, setuper ConnectionSetuper) (net.Listener, error) {
	listener := &tcp{}
	l, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return nil, err
	}
	listener.listener = l
	go runAccept(l, func(rawConn net.Conn) {
		tcpConn := rawConn.(*net.TCPConn)
		setuper.Setup(ctx, Metadata{
			Channel:         tcpConn,
			Encrypted:       false,
			EncryptionState: nil,
			Name:            "tcp",
		})
	})
	return l, nil
}
