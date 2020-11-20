package transport

import (
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"net"
)

type tlsListener struct {
	listener net.Listener
}

func NewTLSTransport(tlsConfig *tls.Config, port int, handler func(Metadata) error) (net.Listener, error) {
	listener := &tlsListener{}

	l, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return nil, err
	}

	listener.listener = l
	go runAccept(l, func(rawConn net.Conn) {
		tcpConn := rawConn.(*net.TCPConn)
		c := tls.Server(tcpConn, tlsConfig)
		err = c.Handshake()
		if err != nil {
			c.Close()
			if err == io.EOF {
				return
			}
			log.Printf("ERROR: tls handshake failed: %v", err)
			return
		}
		state := c.ConnectionState()
		handler(Metadata{
			Channel:         c,
			Encrypted:       true,
			EncryptionState: &state,
			Name:            "tls",
			FD:              socketFD(tcpConn),
		})
	})
	return l, nil
}
