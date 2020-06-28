package transport

import (
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type wsListener struct {
	listener net.Listener
}

var (
	upgrader = websocket.Upgrader{
		Subprotocols:    []string{"mqttv3.1", "mqtt"},
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

func serveWs(cb func(net.Conn)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ws, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			if _, ok := err.(websocket.HandshakeError); !ok {
				log.Println(err)
			}
			return
		}
		cb(&websocketConnector{
			Conn: ws,
		})
	}
}

func NewWSTransport(port int, handler func(Metadata) error) (net.Listener, error) {
	listener := &wsListener{}

	mux := http.NewServeMux()
	mux.HandleFunc("/mqtt", serveWs(func(c net.Conn) {
		listener.queueSession(c, handler)
	}))
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		log.Fatalf("failed to start WS listener: %v", err)
	}
	listener.listener = ln
	go http.Serve(ln, mux)
	return ln, nil
}

func (t *wsListener) queueSession(c net.Conn, handler func(Metadata) error) {
	handler(Metadata{
		Channel:         c,
		Encrypted:       false,
		EncryptionState: nil,
		Name:            "ws",
		RemoteAddress:   c.RemoteAddr().String(),
	})
}
