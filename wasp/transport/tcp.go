package transport

import (
	"fmt"
	"net"

	proxyproto "github.com/armon/go-proxyproto"
)

type tcp struct {
	listener net.Listener
}

func NewTCPTransport(port int, handler func(Metadata) error) (net.Listener, error) {
	listener := &tcp{}
	tcp, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return nil, err
	}
	proxyListener := &proxyproto.Listener{Listener: tcp}
	listener.listener = proxyListener
	go runAccept(proxyListener, func(c net.Conn) {
		listener.queueSession(c, handler)
	})
	return proxyListener, nil
}

func (t *tcp) queueSession(c net.Conn, handler func(Metadata) error) {
	go handler(Metadata{
		Channel:         c,
		Encrypted:       false,
		EncryptionState: nil,
		Name:            "tcp",
		RemoteAddress:   c.RemoteAddr().String(),
	})
}
