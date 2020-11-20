package expiration

import (
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/google/btree"
)

type item struct {
	value    interface{}
	deadline time.Time
}

type bucket struct {
	mtx      sync.Mutex
	index    int
	data     []item
	deadline time.Time
}

func (b *bucket) String() string {
	return fmt.Sprintf("%v (%d)", b.deadline, len(b.data))
}
func (b *bucket) ExtractKey() float64 {
	return float64(b.deadline.Unix())
}
func (b *bucket) Less(i btree.Item) bool {
	return b.deadline.Before(i.(*bucket).deadline)
}
func (b *bucket) put(v interface{}, deadline time.Time) {
	b.mtx.Lock()
	defer b.mtx.Unlock()
	b.data = append(b.data, item{value: v, deadline: deadline})
	sort.SliceStable(b.data, func(i, j int) bool {
		return b.data[i].deadline.Before(b.data[j].deadline)
	})
}
func (b *bucket) delete(v interface{}, deadline time.Time) bool {
	b.mtx.Lock()
	defer b.mtx.Unlock()

	idx := sort.Search(len(b.data), func(i int) bool {
		return !b.data[i].deadline.Before(deadline)
	})
	if idx >= len(b.data) {
		return false
	}

	b.data = append(b.data[:idx], b.data[idx+1:]...)
	return true
}
