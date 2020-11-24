package transport

import (
	"context"
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
	mux.HandleFunc("/mqtt", serveWs(func(c net.Conn, fd int, state tls.ConnectionState) {
		listener.queueSession(c, fd, state, handler)
	}))

	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		log.Fatalf("failed to start WS listener: %v", err)
	}
	listener.listener = ln
	srv := &http.Server{
		Handler: mux,
		ConnContext: func(ctx context.Context, c net.Conn) context.Context {
			tcpConn := c.(*wsConn)
			return context.WithValue(context.WithValue(ctx, connFD, socketFD(tcpConn.initial)), connTLSState, tcpConn.encryptionState)
		},
	}
	go srv.Serve(&wsTLSListener{l: ln, config: tlsConfig})
	return ln, nil
}

func (t *wssListener) queueSession(c net.Conn, fd int, state tls.ConnectionState, handler func(Metadata) error) {
	handler(Metadata{
		Channel:         c,
		Encrypted:       true,
		EncryptionState: &state,
		Name:            "wss",
		RemoteAddress:   c.RemoteAddr().String(),
		FD:              fd,
	})
}
