package transport

import (
	"fmt"
	"net"
)

type tcp struct {
	listener net.Listener
}

func NewTCPTransport(port int, handler func(Metadata) error) (net.Listener, error) {
	listener := &tcp{}
	l, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return nil, err
	}
	listener.listener = l
	go runAccept(l, func(rawConn net.Conn) {
		tcpConn := rawConn.(*net.TCPConn)
		fd, err := tcpConn.File()
		if err != nil {
			rawConn.Close()
			return
		}
		go handler(Metadata{
			Channel:         fd,
			Encrypted:       false,
			EncryptionState: nil,
			Name:            "tcp",
			FD:              int(fd.Fd()),
		})
	})
	return l, nil
}
