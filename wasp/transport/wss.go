package transport

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"time"
)

type wssListener struct {
	srv *http.Server
}

func (l *wssListener) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return l.srv.Shutdown(ctx)
}

func NewWSSTransport(tlsConfig *tls.Config, port int, handler func(Metadata) error) (io.Closer, error) {
	listener := &wssListener{}
	mux := http.NewServeMux()
	mux.HandleFunc("/mqtt", serveWs(func(c net.Conn, state tls.ConnectionState) {
		listener.queueSession(c, state, handler)
	}))
	ln, err := tls.Listen("tcp", fmt.Sprintf(":%d", port), tlsConfig)
	if err != nil {
		log.Fatalf("failed to start WSS listener: %v", err)
	}
	srv := &http.Server{
		Handler: mux,
	}
	go func() {
		err := srv.Serve(ln)
		if err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()
	listener.srv = srv
	return listener, nil
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
