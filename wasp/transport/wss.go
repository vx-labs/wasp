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
	mux.HandleFunc("/mqtt", serveWs(func(c net.Conn, fd int) {
		listener.queueSession(c, fd, handler)
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
			fd, err := tcpConn.initial.(*net.TCPConn).File()
			if err != nil {
				panic(err)
			}
			return context.WithValue(ctx, connFD, int(fd.Fd()))
		},
	}
	go srv.Serve(&wsTLSListener{l: ln, config: tlsConfig})
	return ln, nil
}

func (t *wssListener) queueSession(c net.Conn, fd int, handler func(Metadata) error) {
	handler(Metadata{
		Channel:         c,
		Encrypted:       true,
		EncryptionState: nil,
		Name:            "wss",
		RemoteAddress:   c.RemoteAddr().String(),
		FD:              fd,
	})
}
