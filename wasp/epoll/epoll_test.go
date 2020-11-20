package epoll

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/vx-labs/wasp/wasp/expiration"
	"golang.org/x/sys/unix"
)

func BenchmarkEpoll(b *testing.B) {
	b.StopTimer()
	e := &instance{
		fd:          -1,
		connections: &db{conns: make(map[int]*ClientConn)},
		events:      make([]unix.EpollEvent, 100),
		timeouts:    expiration.NewList(),
	}

	for i := 0; i < 1000; i++ {
		require.NoError(b, e.Add(ClientConn{
			ID:       fmt.Sprintf("%d", i),
			Deadline: time.Time{}.Add(time.Hour),
			FD:       i,
		}))
	}
	for i := 0; i < 100; i++ {
		e.events[i] = unix.EpollEvent{Fd: int32(i), Events: unix.EPOLLIN}
	}
	in := make([]Event, 100)
	b.ResetTimer()
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		e.processEvents(100, in)
	}
}
