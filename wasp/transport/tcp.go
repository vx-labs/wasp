package transport

import (
	"fmt"
	"net"
)

type tcp struct {
	listener net.Listener
}

func NewTCPTransport(port int, handler func(Metadata) error) (net.Listener, error) {
	listener := &tcp{}
	l, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return nil, err
	}
	listener.listener = l
	go runAccept(l, func(rawConn net.Conn) {
		tcpConn := rawConn.(*net.TCPConn)
		handler(Metadata{
			Channel:         tcpConn,
			Encrypted:       false,
			EncryptionState: nil,
			Name:            "tcp",
			FD:              socketFD(tcpConn),
		})
	})
	return l, nil
}
