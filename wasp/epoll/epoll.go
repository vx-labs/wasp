package epoll

import (
	"errors"
	"sync"
	"syscall"
	"time"

	"github.com/vx-labs/wasp/wasp/expiration"
	"github.com/vx-labs/wasp/wasp/transport"
	"github.com/zond/gotomic"
	"golang.org/x/sys/unix"
)

type ClientConn struct {
	ID       string
	FD       int
	Conn     transport.TimeoutReadWriteCloser
	Deadline time.Time
}

var (
	ErrConnectionAlreadyExists = errors.New("connection already exists")
	ErrConnectionNotFound      = errors.New("connection not found")
)

type Epoll struct {
	fd          int
	connections *gotomic.Hash
	lock        *sync.RWMutex
	events      []unix.EpollEvent
	timeouts    expiration.List
}

func New(maxEvents int) (*Epoll, error) {
	fd, err := unix.EpollCreate1(0)
	if err != nil {
		return nil, err
	}
	return &Epoll{
		fd:          fd,
		lock:        &sync.RWMutex{},
		connections: gotomic.NewHash(),
		events:      make([]unix.EpollEvent, maxEvents),
		timeouts:    expiration.NewList(),
	}, nil
}

var epollEvents uint32 = unix.POLLIN | unix.POLLHUP | unix.EPOLLONESHOT

func (e *Epoll) Expire(now time.Time) []ClientConn {
	expired := e.timeouts.Expire(now)
	out := make([]ClientConn, 0, len(expired))
	for _, v := range expired {
		v, ok := e.connections.Delete(v.(gotomic.Hashable))
		if ok {
			c := v.(ClientConn)
			c.Conn.Close()
			out = append(out, c)
		}
	}
	return out
}
func (e *Epoll) SetDeadline(fd int, deadline time.Time) {
	k := gotomic.IntKey(fd)
	v, ok := e.connections.Get(k)
	if ok {
		conn := v.(ClientConn)
		e.timeouts.Update(k, conn.Deadline, deadline)
		conn.Deadline = deadline
		e.connections.Put(k, conn)
	}
}

func (e *Epoll) Add(conn ClientConn) error {
	k := gotomic.IntKey(conn.FD)
	ok := e.connections.PutIfMissing(k, conn)
	if !ok {
		return ErrConnectionAlreadyExists
	}
	err := unix.EpollCtl(e.fd, syscall.EPOLL_CTL_ADD, conn.FD, &unix.EpollEvent{Events: epollEvents, Fd: int32(conn.FD)})
	if err != nil {
		e.connections.Delete(k)
		return err
	}
	if !conn.Deadline.IsZero() {
		e.timeouts.Insert(gotomic.IntKey(conn.FD), conn.Deadline)
	}
	return nil
}

func (e *Epoll) Remove(fd int) error {
	err := unix.EpollCtl(e.fd, syscall.EPOLL_CTL_DEL, fd, nil)
	if err != nil {
		return err
	}
	k := gotomic.IntKey(fd)
	v, ok := e.connections.Delete(k)
	if !ok {
		return ErrConnectionNotFound
	}
	conn := v.(ClientConn)
	if !conn.Deadline.IsZero() {
		e.timeouts.Delete(k, conn.Deadline)
	}
	return nil
}
func (e *Epoll) Rearm(fd int) error {
	return unix.EpollCtl(e.fd, syscall.EPOLL_CTL_MOD, fd, &unix.EpollEvent{Events: epollEvents, Fd: int32(fd)})
}

func (e *Epoll) Wait(connections []ClientConn) (int, error) {
	n, err := unix.EpollWait(e.fd, e.events, 100)
	if err != nil {
		return 0, err
	}
	for i := 0; i < n; i++ {
		k := gotomic.IntKey(e.events[i].Fd)
		conn, ok := e.connections.Get(k)
		if ok {
			connections[i] = conn.(ClientConn)
		}
	}
	return n, nil
}

func (e *Epoll) Shutdown() {
	e.timeouts.Reset()
	unix.Close(e.fd)
	e.connections.Each(func(k gotomic.Hashable, _ gotomic.Thing) bool {
		v, ok := e.connections.Delete(k)
		if ok {
			v.(ClientConn).Conn.Close()
		}
		return false
	})
}
