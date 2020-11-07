package transport

import (
	"crypto/tls"
	"fmt"
	"log"
	"net"

	"github.com/armon/go-proxyproto"
)

type tlsListener struct {
	listener net.Listener
}

func NewTLSTransport(tlsConfig *tls.Config, port int, handler func(Metadata) error) (net.Listener, error) {
	listener := &tlsListener{}

	tcp, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return nil, err
	}
	proxyList := &proxyproto.Listener{Listener: tcp}

	l := tls.NewListener(proxyList, tlsConfig)
	listener.listener = l
	go runAccept(l, func(rawConn net.Conn) {
		c, ok := rawConn.(*tls.Conn)
		if !ok {
			c.Close()
			return
		}
		err := c.Handshake()
		if err != nil {
			log.Printf("ERROR: tls handshake failed: %v", err)
			c.Close()
			return
		}
		listener.queueSession(c, handler)
	})
	return l, nil
}

func (t *tlsListener) queueSession(c *tls.Conn, handler func(Metadata) error) {
	state := c.ConnectionState()
	go handler(Metadata{
		Channel:         c,
		Encrypted:       true,
		EncryptionState: &state,
		Name:            "tls",
		RemoteAddress:   c.RemoteAddr().String(),
	})
}
