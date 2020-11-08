package epoll

import (
	"sync"
	"syscall"
	"time"

	"github.com/google/btree"

	"github.com/vx-labs/wasp/wasp/transport"
	"golang.org/x/sys/unix"
)

type ClientConn struct {
	ID       string
	FD       int
	Conn     transport.TimeoutReadWriteCloser
	Deadline time.Time
}

type expirationSet struct {
	data     map[int]struct{}
	deadline time.Time
}

func (e *expirationSet) Less(b btree.Item) bool {
	return e.deadline.Before(b.(*expirationSet).deadline)
}

type Epoll struct {
	fd          int
	connections map[int]*ClientConn
	lock        *sync.RWMutex
	events      []unix.EpollEvent
	timeouts    *btree.BTree
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
		timeouts:    btree.New(2),
	}, nil
}

var epollEvents uint32 = unix.POLLIN | unix.POLLHUP | unix.EPOLLONESHOT

func (e *Epoll) Expire(now time.Time) {
	e.lock.Lock()
	defer e.lock.Unlock()
	deleted := []time.Time{}
	e.timeouts.AscendLessThan(&expirationSet{deadline: now}, func(i btree.Item) bool {
		set := i.(*expirationSet)
		for fd := range set.data {
			if e.connections[fd] != nil && e.connections[fd].Conn != nil {
				e.connections[fd].Conn.Close()
			}
		}
		deleted = append(deleted, set.deadline)
		return true
	})
	for _, t := range deleted {
		e.timeouts.Delete(&expirationSet{deadline: t})
	}
}
func (e *Epoll) SetDeadline(fd int, deadline time.Time) {
	e.lock.Lock()
	defer e.lock.Unlock()
	conn := e.connections[fd]

	if !conn.Deadline.IsZero() {
		set := e.timeouts.Get(&expirationSet{deadline: conn.Deadline.Round(time.Second)})
		delete(set.(*expirationSet).data, conn.FD)
		if len(set.(*expirationSet).data) == 0 {
			e.timeouts.Delete(&expirationSet{deadline: conn.Deadline.Round(time.Second)})
		}
	}
	conn.Deadline = deadline
	set := e.timeouts.Get(&expirationSet{deadline: deadline.Round(time.Second)})
	if set == nil {
		set = &expirationSet{data: make(map[int]struct{}), deadline: deadline.Round(time.Second)}
	}
	set.(*expirationSet).data[conn.FD] = struct{}{}
	e.timeouts.ReplaceOrInsert(set)
}

func (e *Epoll) Add(conn *ClientConn) error {
	err := unix.EpollCtl(e.fd, syscall.EPOLL_CTL_ADD, conn.FD, &unix.EpollEvent{Events: epollEvents, Fd: int32(conn.FD)})
	if err != nil {
		return err
	}
	e.lock.Lock()
	defer e.lock.Unlock()
	e.connections[conn.FD] = conn
	if !conn.Deadline.IsZero() {
		set := e.timeouts.Get(&expirationSet{deadline: conn.Deadline.Round(time.Second)})
		if set == nil {
			set = &expirationSet{data: make(map[int]struct{}), deadline: conn.Deadline.Round(time.Second)}
		}
		set.(*expirationSet).data[conn.FD] = struct{}{}
		e.timeouts.ReplaceOrInsert(set)
	}
	return nil
}

func (e *Epoll) Remove(fd int) error {
	err := unix.EpollCtl(e.fd, syscall.EPOLL_CTL_DEL, fd, nil)
	if err != nil {
		return err
	}
	e.lock.Lock()
	defer e.lock.Unlock()
	conn := e.connections[fd]
	if !conn.Deadline.IsZero() {
		set := e.timeouts.Get(&expirationSet{deadline: conn.Deadline.Round(time.Second)})
		if set != nil {
			delete(set.(*expirationSet).data, fd)
		}
	}
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

func (e *Epoll) Shutdown() {
	e.lock.Lock()
	defer e.lock.Unlock()
	unix.Close(e.fd)
	for fd, conn := range e.connections {
		delete(e.connections, fd)
		conn.Conn.Close()
	}
	e.timeouts.Clear(false)
}
