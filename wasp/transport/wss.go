package transport

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
)

type wssListener struct {
	listener net.Listener
}

func NewWSSTransport(tlsConfig *tls.Config, port int, handler func(Metadata) error) (net.Listener, error) {
	listener := &wssListener{}
	mux := http.NewServeMux()
	mux.HandleFunc("/mqtt", serveWs(func(c net.Conn) {
		listener.queueSession(c, handler)
	}))

	ln, err := tls.Listen("tcp", fmt.Sprintf(":%d", port), tlsConfig)
	if err != nil {
		return nil, err
	}
	listener.listener = ln
	go http.Serve(ln, mux)
	return ln, nil
}

func (t *wssListener) queueSession(c net.Conn, handler func(Metadata) error) {
	handler(Metadata{
		Channel:         c,
		Encrypted:       true,
		EncryptionState: nil,
		Name:            "wss",
		RemoteAddress:   c.RemoteAddr().String(),
	})
}
