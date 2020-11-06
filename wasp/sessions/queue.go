package sessions

import (
	"sync"
)

type QueueIterator struct {
	c    *sync.Cond
	data []uint64
}

func NewQueueIterator() *QueueIterator {
	mtx := sync.Mutex{}
	return &QueueIterator{
		c: sync.NewCond(&mtx),
	}
}

func (t *QueueIterator) AdvanceTo(v uint64) {
	t.c.L.Lock()
	defer t.c.L.Unlock()
	for idx := range t.data {
		if t.data[idx] == v {
			t.data = t.data[idx:]
			return
		}
	}
	t.data = []uint64{}
}
func (t *QueueIterator) Next() (uint64, error) {
	t.c.L.Lock()
	defer t.c.L.Unlock()
	for len(t.data) == 0 {
		t.c.Wait()
	}
	value := t.data[0]
	t.data = t.data[1:]
	return value, nil
}
