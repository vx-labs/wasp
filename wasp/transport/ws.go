package transport

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type contextKey string

const connFD contextKey = "wasp.connFD"
const connTLSState contextKey = "wasp.connTLSState"

type wsListener struct {
	listener net.Listener
}

var (
	upgrader = websocket.Upgrader{
		Subprotocols:    []string{"mqttv3.1", "mqtt"},
		CheckOrigin:     func(r *http.Request) bool { return true },
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
	}
)

// websocketConnector is a websocket wrapper so it satisfies the net.Conn interface so it is a
// drop in replacement of the golang.org/x/net/websocket package.
// Implementation taken from https://github.com/eclipse/paho.mqtt.golang/blob/master/websocket.go
type websocketConnector struct {
	*websocket.Conn
	r   io.Reader
	rio sync.Mutex
	wio sync.Mutex
}

// SetDeadline sets both the read and write deadlines
func (c *websocketConnector) SetDeadline(t time.Time) error {
	if err := c.SetReadDeadline(t); err != nil {
		return err
	}
	err := c.SetWriteDeadline(t)
	return err
}

// Write writes data to the websocket
func (c *websocketConnector) Write(p []byte) (int, error) {
	c.wio.Lock()
	defer c.wio.Unlock()

	err := c.WriteMessage(websocket.BinaryMessage, p)
	if err != nil {
		return 0, err
	}
	return len(p), nil
}

// Read reads the current websocket frame
func (c *websocketConnector) Read(p []byte) (int, error) {
	c.rio.Lock()
	defer c.rio.Unlock()
	for {
		if c.r == nil {
			// Advance to next message.
			var err error
			_, c.r, err = c.NextReader()
			if err != nil {
				return 0, err
			}
		}
		n, err := c.r.Read(p)
		if err == io.EOF {
			// At end of message.
			c.r = nil
			if n > 0 {
				return n, nil
			}
			// No data read, continue to next message.
			continue
		}
		return n, err
	}
}

func serveWs(cb func(net.Conn, tls.ConnectionState)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ws, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Println(err)
			return
		}
		tlsStateKey := r.Context().Value(connTLSState)
		if tlsStateKey != nil {
			cb(&websocketConnector{
				Conn: ws,
			}, tlsStateKey.(tls.ConnectionState))
		} else {
			cb(&websocketConnector{
				Conn: ws,
			}, tls.ConnectionState{})
		}
	}
}

func NewWSTransport(ctx context.Context, port int, setuper ConnectionSetuper) (net.Listener, error) {
	listener := &wsListener{}

	mux := http.NewServeMux()
	mux.HandleFunc("/mqtt", serveWs(func(c net.Conn, _ tls.ConnectionState) {
		listener.queueSession(ctx, c, setuper)
	}))
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		log.Fatalf("failed to start WS listener: %v", err)
	}
	listener.listener = ln
	srv := &http.Server{
		Handler: mux,
	}
	go srv.Serve(ln)
	return ln, nil
}

func (t *wsListener) queueSession(ctx context.Context, c net.Conn, setuper ConnectionSetuper) {
	setuper.Setup(ctx, Metadata{
		Channel:         c,
		Encrypted:       false,
		EncryptionState: nil,
		Name:            "ws",
		RemoteAddress:   c.RemoteAddr().String(),
	})
}
