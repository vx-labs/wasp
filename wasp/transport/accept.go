package transport

import (
	"fmt"
	"log"
	"net"
	"time"
)

func runAccept(l net.Listener, handler func(c net.Conn)) error {
	var tempDelay time.Duration
	for {
		rawConn, err := l.Accept()

		if err != nil {
			if err.Error() == fmt.Sprintf("accept tcp %v: use of closed network connection", l.Addr()) {
				return err
			}
			if ne, ok := err.(net.Error); ok && ne.Temporary() {
				if tempDelay == 0 {
					tempDelay = 5 * time.Millisecond
				} else {
					tempDelay *= 2
				}
				if max := 1 * time.Second; tempDelay > max {
					tempDelay = max
				}
				log.Printf("WARN: accept error: %v; retrying in %v", err, tempDelay)
				time.Sleep(tempDelay)
				continue
			}
			log.Printf("ERROR: connection handling failed: %v", err)
			return l.Close()
		}
		handler(rawConn)
	}
}
