package epoll

import (
	"errors"
	"io"
	"sync"
	"syscall"
	"time"

	"github.com/vx-labs/wasp/v4/wasp/expiration"
	"github.com/vx-labs/wasp/v4/wasp/transport"
	"github.com/zond/gotomic"
	"golang.org/x/sys/unix"
)

// ClientConn represents a network connection been tracked
type ClientConn struct {
	ID       string
	FD       int
	Conn     transport.TimeoutReadWriteCloser
	Deadline time.Time
}

// Event represents a network event and the associated client connection
type Event struct {
	ID    string
	FD    int
	Conn  io.ReadWriter
	Event uint32
}

const epollEvents uint32 = unix.POLLIN | unix.POLLHUP | unix.EPOLLONESHOT | unix.EPOLLRDHUP

var (
	// ErrConnectionAlreadyExists means that the connection is already tracked
	ErrConnectionAlreadyExists = errors.New("connection already exists")
	//ErrConnectionNotFound means that the connection was not found
	ErrConnectionNotFound = errors.New("connection not found")
)

// Instance tracks file descriptors and notify when data is ready to be read
type Instance interface {
	Expire(now time.Time) []ClientConn
	SetDeadline(fd int, deadline time.Time)
	Add(conn ClientConn) error
	Remove(fd int) error
	Rearm(fd int) error
	Wait(connections []Event) (int, error)
	Shutdown()
	Count() int
}
type instance struct {
	fd          int
	mtx         sync.RWMutex
	connections *db
	events      []unix.EpollEvent
	timeouts    expiration.List
}

// NewInstance returns a new epoll instance
func NewInstance(maxEvents int) (Instance, error) {
	fd, err := unix.EpollCreate1(0)
	if err != nil {
		return nil, err
	}
	return &instance{
		fd:          fd,
		connections: &db{conns: make(map[int]*ClientConn)},
		events:      make([]unix.EpollEvent, maxEvents),
		timeouts:    expiration.NewList(),
	}, nil
}

func (e *instance) Count() int {
	e.mtx.RLock()
	defer e.mtx.RUnlock()
	return e.connections.Len()
}

func (e *instance) Expire(now time.Time) []ClientConn {
	e.mtx.Lock()
	defer e.mtx.Unlock()

	expired := e.timeouts.Expire(now)
	out := make([]ClientConn, 0, len(expired))
	for _, v := range expired {
		c, ok := e.connections.Get(v.(int))
		if ok {
			e.connections.Delete(c.FD)
			c.Conn.Close()
			out = append(out, *c)
		}
	}
	return out
}
func (e *instance) SetDeadline(fd int, deadline time.Time) {
	e.mtx.RLock()
	defer e.mtx.RUnlock()

	v, ok := e.connections.Get(fd)
	if ok {
		conn := v
		e.timeouts.Update(fd, conn.Deadline, deadline)
		conn.Deadline = deadline
		e.connections.Update(conn)
	}
}

func (e *instance) addEpoll(conn ClientConn) error {
	if e.fd == -1 {
		// used by test cases
		return nil
	}
	return unix.EpollCtl(e.fd, syscall.EPOLL_CTL_ADD, conn.FD, &unix.EpollEvent{Events: epollEvents, Fd: int32(conn.FD)})
}

func (e *instance) Add(conn ClientConn) error {
	e.mtx.Lock()
	defer e.mtx.Unlock()

	_, ok := e.connections.Get(conn.FD)
	if ok {
		return ErrConnectionAlreadyExists
	}
	e.connections.Insert(&conn)
	err := e.addEpoll(conn)
	if err != nil {
		e.connections.Delete(conn.FD)
		return err
	}
	if !conn.Deadline.IsZero() {
		e.timeouts.Insert(gotomic.IntKey(conn.FD), conn.Deadline)
	}
	return nil
}

func (e *instance) Remove(fd int) error {
	e.mtx.Lock()
	defer e.mtx.Unlock()

	conn, ok := e.connections.Get(fd)
	if !ok {
		return ErrConnectionNotFound
	}
	e.connections.Delete(fd)
	if !conn.Deadline.IsZero() {
		e.timeouts.Delete(fd, conn.Deadline)
	}
	conn.Conn.Close()
	return nil
}
func (e *instance) Rearm(fd int) error {
	return unix.EpollCtl(e.fd, syscall.EPOLL_CTL_MOD, fd, &unix.EpollEvent{Events: epollEvents, Fd: int32(fd)})
}

func (e *instance) processEvents(n int, connections []Event) (int, error) {
	for i := 0; i < n; i++ {
		conn, ok := e.connections.Get(int(e.events[i].Fd))
		if ok {
			connections[i] = Event{
				ID:    conn.ID,
				FD:    conn.FD,
				Conn:  conn.Conn,
				Event: e.events[i].Events,
			}
		}
	}
	return n, nil
}
func (e *instance) Wait(connections []Event) (int, error) {
	e.mtx.RLock()
	defer e.mtx.RUnlock()

	n, err := unix.EpollWait(e.fd, e.events, 100)
	if err != nil {
		return 0, err
	}
	return e.processEvents(n, connections)
}

func (e *instance) Shutdown() {
	e.mtx.Lock()
	defer e.mtx.Unlock()

	e.timeouts.Reset()
	unix.Close(e.fd)

	e.connections.Range(func(conn *ClientConn) bool {
		conn.Conn.Close()
		return false
	})
	e.connections = nil
}
