package transport

import (
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"net/http"
)

type wssListener struct {
	listener net.Listener
}

func NewWSSTransport(tlsConfig *tls.Config, port int, handler func(Metadata) error) (net.Listener, error) {
	listener := &wssListener{}
	mux := http.NewServeMux()
	mux.HandleFunc("/mqtt", serveWs(func(c net.Conn, state tls.ConnectionState) {
		listener.queueSession(c, state, handler)
	}))

	ln, err := tls.Listen("tcp", fmt.Sprintf(":%d", port), tlsConfig)
	if err != nil {
		log.Fatalf("failed to start WSS listener: %v", err)
	}
	listener.listener = ln
	srv := &http.Server{
		Handler: mux,
	}
	go srv.Serve(ln)
	return ln, nil
}

func (t *wssListener) queueSession(c net.Conn, state tls.ConnectionState, handler func(Metadata) error) {
	handler(Metadata{
		Channel:         c,
		Encrypted:       true,
		EncryptionState: &state,
		Name:            "wss",
		RemoteAddress:   c.RemoteAddr().String(),
	})
}
