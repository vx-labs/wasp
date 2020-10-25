package sessions

import (
	"sync"
)

type queueIterator struct {
	c    *sync.Cond
	data []uint64
}

func (t *queueIterator) AdvanceTo(v uint64) {
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
func (t *queueIterator) Push(id uint64) {
	t.c.L.Lock()
	defer t.c.L.Unlock()
	t.data = append(t.data, id)
	t.c.Broadcast()
}
func (t *queueIterator) Next() (uint64, error) {
	t.c.L.Lock()
	defer t.c.L.Unlock()
	for len(t.data) == 0 {
		t.c.Wait()
	}
	value := t.data[0]
	t.data = t.data[1:]
	return value, nil
}
