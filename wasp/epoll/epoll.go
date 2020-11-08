package epoll

import (
	"sync"
	"syscall"

	"github.com/vx-labs/wasp/wasp/transport"
	"golang.org/x/sys/unix"
)

type ClientConn struct {
	ID   string
	FD   int
	Conn transport.TimeoutReadWriteCloser
}

type Epoll struct {
	fd          int
	connections map[int]*ClientConn
	lock        *sync.RWMutex
	events      []unix.EpollEvent
}

func New(maxEvents int) (*Epoll, error) {
	fd, err := unix.EpollCreate1(0)
	if err != nil {
		return nil, err
	}
	return &Epoll{
		fd:          fd,
		lock:        &sync.RWMutex{},
		connections: make(map[int]*ClientConn),
		events:      make([]unix.EpollEvent, maxEvents),
	}, nil
}

var epollEvents uint32 = unix.POLLIN | unix.POLLHUP | unix.EPOLLONESHOT

func (e *Epoll) Add(id string, fd int, t transport.TimeoutReadWriteCloser) error {
	err := unix.EpollCtl(e.fd, syscall.EPOLL_CTL_ADD, fd, &unix.EpollEvent{Events: epollEvents, Fd: int32(fd)})
	if err != nil {
		return err
	}
	e.lock.Lock()
	defer e.lock.Unlock()
	e.connections[fd] = &ClientConn{ID: id, FD: fd, Conn: t}
	return nil
}

func (e *Epoll) Remove(fd int) error {
	err := unix.EpollCtl(e.fd, syscall.EPOLL_CTL_DEL, fd, nil)
	if err != nil {
		return err
	}
	e.lock.Lock()
	defer e.lock.Unlock()
	delete(e.connections, fd)
	return nil
}
func (e *Epoll) Rearm(fd int) error {
	return unix.EpollCtl(e.fd, syscall.EPOLL_CTL_MOD, fd, &unix.EpollEvent{Events: epollEvents, Fd: int32(fd)})
}

func (e *Epoll) Wait(connections []*ClientConn) (int, error) {
	n, err := unix.EpollWait(e.fd, e.events, 100)
	if err != nil {
		return 0, err
	}
	e.lock.RLock()
	defer e.lock.RUnlock()
	for i := 0; i < n; i++ {
		conn := e.connections[int(e.events[i].Fd)]
		connections[i] = conn
	}
	return n, nil
}
