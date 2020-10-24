package sessions

import (
	"io"
	"sync"
)

type queueIterator struct {
	mtx  sync.Mutex
	data []uint64
}

func (t *queueIterator) AdvanceTo(v uint64) {
	t.mtx.Lock()
	defer t.mtx.Unlock()
	for idx := range t.data {
		if t.data[idx] == v {
			t.data = t.data[idx:]
			return
		}
	}
	t.data = []uint64{}
}
func (t *queueIterator) Push(id uint64) {
	t.mtx.Lock()
	defer t.mtx.Unlock()
	t.data = append(t.data, id)
}
func (t *queueIterator) Next() (uint64, error) {
	t.mtx.Lock()
	defer t.mtx.Unlock()
	if len(t.data) == 0 {
		return 0, io.EOF
	}
	value := t.data[0]
	t.data = t.data[1:]
	return value, nil
}
