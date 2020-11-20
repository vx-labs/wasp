package expiration

import (
	"container/heap"
	"sync"
	"time"
)

// A pqueue implements heap.Interface and holds Items.
type pqueue []*bucket

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
	item := x.(*bucket)
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
func (pq *pqueue) Peek() *bucket {
	i := *pq
	if len(i) < 1 {
		return nil
	}
	return i[0]
}

func newPQList() List {
	list := &pqList{
		buckets: make(map[time.Time]*bucket),
		pq:      make(pqueue, 0),
	}
	heap.Init(&list.pq)
	return list
}

type pqList struct {
	mtx     sync.RWMutex
	pq      pqueue
	buckets map[time.Time]*bucket
}

func (pq *pqList) Insert(id interface{}, expireAt time.Time) {
	pq.insert(id, expireAt)
}
func (pq *pqList) insert(id interface{}, expireAt time.Time) {
	pq.mtx.RLock()
	deadline := expireAt.Round(time.Second)
	elt, ok := pq.buckets[deadline]
	pq.mtx.RUnlock()
	if !ok {
		pq.mtx.Lock()
		defer pq.mtx.Unlock()
		if elt, ok = pq.buckets[deadline]; !ok {
			elt = &bucket{
				data: []item{
					{value: id, deadline: expireAt},
				},
				deadline: deadline,
			}
			pq.buckets[deadline] = elt
			heap.Push(&pq.pq, elt)
			return
		}
	}
	elt.put(id, expireAt)
}
func (pq *pqList) Delete(id interface{}, expireAt time.Time) bool {
	return pq.delete(id, expireAt)
}
func (pq *pqList) delete(id interface{}, expireAt time.Time) bool {
	pq.mtx.RLock()
	defer pq.mtx.RUnlock()
	deadline := expireAt.Round(time.Second)
	bucket, ok := pq.buckets[deadline]
	if !ok {
		return false
	}
	bucket.delete(id, expireAt)
	return true
}

func (pq *pqList) Update(id interface{}, old time.Time, new time.Time) {
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
		expired := heap.Pop(&pq.pq).(*bucket)
		for _, v := range expired.data {
			out = append(out, v.value)
		}
	}
}

func (pq *pqList) Reset() {
	pq.mtx.Lock()
	defer pq.mtx.Unlock()

	pq.pq = nil
	pq.buckets = nil
}
