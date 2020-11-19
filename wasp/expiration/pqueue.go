package expiration

import (
	"container/heap"
	"sync"
	"time"

	"github.com/zond/gotomic"
)

// A pqueue implements heap.Interface and holds Items.
type pqueue []*mapBucket

func (pq pqueue) Len() int { return len(pq) }

func (pq pqueue) Less(i, j int) bool {
	return pq[i].deadline.Before(pq[j].deadline)
}

func (pq pqueue) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
	pq[i].index = i
	pq[j].index = j
}

func (pq *pqueue) Push(x interface{}) {
	n := len(*pq)
	item := x.(*mapBucket)
	item.index = n
	*pq = append(*pq, item)
}

func (pq *pqueue) Pop() interface{} {
	old := *pq
	n := len(old)
	item := old[n-1]
	old[n-1] = nil  // avoid memory leak
	item.index = -1 // for safety
	*pq = old[0 : n-1]
	return item
}
func (pq *pqueue) Peek() *mapBucket {
	i := *pq
	if len(i) < 1 {
		return nil
	}
	return i[0]
}

func newPQList() List {
	list := &pqList{
		buckets: make(map[time.Time]*mapBucket),
		pq:      make(pqueue, 0),
	}
	heap.Init(&list.pq)
	return list
}

type pqList struct {
	mtx     sync.RWMutex
	pq      pqueue
	buckets map[time.Time]*mapBucket
}

func (pq *pqList) Insert(id gotomic.Hashable, expireAt time.Time) {
	pq.mtx.Lock()
	defer pq.mtx.Unlock()
	pq.insert(id, expireAt)
}
func (pq *pqList) insert(id gotomic.Hashable, expireAt time.Time) {
	deadline := expireAt.Round(time.Second)
	bucket, ok := pq.buckets[deadline]
	if !ok {
		bucket = &mapBucket{
			data: map[interface{}]struct{}{
				id: {},
			},
			deadline: deadline,
		}
		pq.buckets[deadline] = bucket
		heap.Push(&pq.pq, bucket)
	} else {
		bucket.data[id] = struct{}{}
	}
}
func (pq *pqList) Delete(id gotomic.Hashable, expireAt time.Time) bool {
	pq.mtx.Lock()
	defer pq.mtx.Unlock()
	return pq.delete(id, expireAt)
}
func (pq *pqList) delete(id gotomic.Hashable, expireAt time.Time) bool {
	deadline := expireAt.Round(time.Second)
	bucket, ok := pq.buckets[deadline]
	if !ok {
		return false
	}
	delete(bucket.data, id)
	if len(bucket.data) == 0 {
		heap.Remove(&pq.pq, bucket.index)
		delete(pq.buckets, deadline)
	}
	return true
}

func (pq *pqList) Update(id gotomic.Hashable, old time.Time, new time.Time) {
	pq.mtx.Lock()
	defer pq.mtx.Unlock()
	if pq.delete(id, old) {
		pq.insert(id, new)
	}
}

func (pq *pqList) Expire(now time.Time) []interface{} {
	pq.mtx.Lock()
	defer pq.mtx.Unlock()
	out := []interface{}{}
	for {
		v := pq.pq.Peek()
		if v == nil || !v.deadline.Before(now) {
			return out
		}

		expired := heap.Pop(&pq.pq).(*mapBucket)
		for v := range expired.data {
			out = append(out, v)
		}
	}
}

func (pq *pqList) Reset() {
	pq.mtx.Lock()
	defer pq.mtx.Unlock()

	pq.pq = nil
	pq.buckets = nil
}
