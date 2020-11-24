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

func serveWs(cb func(net.Conn, int, tls.ConnectionState)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ws, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Println(err)
			return
		}
		cb(&websocketConnector{
			Conn: ws,
		}, r.Context().Value(connFD).(int), r.Context().Value(connTLSState).(tls.ConnectionState))
	}
}

type wsConn struct {
	backend         net.Conn
	initial         net.Conn
	encryptionState tls.ConnectionState
}

func (w *wsConn) SetWriteDeadline(t time.Time) error { return w.backend.SetWriteDeadline(t) }
func (w *wsConn) SetReadDeadline(t time.Time) error  { return w.backend.SetReadDeadline(t) }
func (w *wsConn) SetDeadline(t time.Time) error      { return w.backend.SetDeadline(t) }
func (w *wsConn) Read(b []byte) (int, error)         { return w.backend.Read(b) }
func (w *wsConn) Write(b []byte) (int, error)        { return w.backend.Write(b) }
func (w *wsConn) Close() error                       { return w.backend.Close() }
func (w *wsConn) LocalAddr() net.Addr                { return w.backend.LocalAddr() }
func (w *wsConn) RemoteAddr() net.Addr               { return w.backend.RemoteAddr() }

type wsTCPListener struct {
	l net.Listener
}

func (w *wsTCPListener) Accept() (net.Conn, error) {
	c, err := w.l.Accept()
	return &wsConn{backend: c, initial: c}, err
}

func (w *wsTCPListener) Close() error {
	return w.l.Close()
}
func (w *wsTCPListener) Addr() net.Addr {
	return w.l.Addr()
}

type wsTLSListener struct {
	l      net.Listener
	config *tls.Config
}

func (w *wsTLSListener) Accept() (net.Conn, error) {
	c, err := w.l.Accept()
	if err != nil {
		return nil, err
	}
	tlsConn := tls.Server(c, w.config)
	return &wsConn{backend: tlsConn, initial: c, encryptionState: tlsConn.ConnectionState()}, err
}

func (w *wsTLSListener) Close() error {
	return w.l.Close()
}
func (w *wsTLSListener) Addr() net.Addr {
	return w.l.Addr()
}

func NewWSTransport(port int, handler func(Metadata) error) (net.Listener, error) {
	listener := &wsListener{}

	mux := http.NewServeMux()
	mux.HandleFunc("/mqtt", serveWs(func(c net.Conn, fd int, _ tls.ConnectionState) {
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
			return context.WithValue(ctx, connFD, socketFD(tcpConn.initial))
		},
	}
	go srv.Serve(&wsTCPListener{l: ln})
	return ln, nil
}

func (t *wsListener) queueSession(c net.Conn, fd int, handler func(Metadata) error) {
	handler(Metadata{
		Channel:         c,
		Encrypted:       false,
		EncryptionState: nil,
		Name:            "ws",
		RemoteAddress:   c.RemoteAddr().String(),
		FD:              fd,
	})
}
