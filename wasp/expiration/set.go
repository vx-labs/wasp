package expiration

import (
	"sync"
	"time"

	"github.com/MauriceGit/skiplist"
	"github.com/google/btree"
)

type bucket struct {
	data     map[int]struct{}
	deadline time.Time
}

func (e *bucket) Less(b btree.Item) bool {
	return e.deadline.Before(b.(*bucket).deadline)
}
func (e *bucket) ExtractKey() float64 {
	return float64(e.deadline.Unix())
}
func (e *bucket) String() string {
	return e.deadline.String()
}

type List interface {
	Insert(id int, deadline time.Time)
	Delete(id int, deadline time.Time)
	Update(id int, old time.Time, new time.Time)
	Expire(now time.Time) []int
	Reset()
}

type list struct {
	mtx  sync.Mutex
	tree skiplist.SkipList
}

func NewList() *list {
	return &list{tree: skiplist.New()}
}

func (l *list) Insert(id int, deadline time.Time) {
	l.mtx.Lock()
	defer l.mtx.Unlock()
	l.insert(id, deadline)
}

func (l *list) Reset() {
	l.tree = skiplist.New()
}
func (l *list) Delete(id int, deadline time.Time) {
	l.delete(id, deadline)
}
func (l *list) delete(id int, deadline time.Time) {
	set, ok := l.tree.Find(&bucket{deadline: deadline.Round(time.Second)})
	if ok {
		delete(set.GetValue().(*bucket).data, id)
		if len(set.GetValue().(*bucket).data) == 0 {
			l.tree.Delete(&bucket{deadline: deadline.Round(time.Second)})
		}
	}
}
func (l *list) insert(id int, deadline time.Time) {
	var b *bucket
	set, ok := l.tree.Find(&bucket{deadline: deadline.Round(time.Second)})
	if !ok {
		b = &bucket{data: make(map[int]struct{}), deadline: deadline.Round(time.Second)}
	} else {
		b = set.GetValue().(*bucket)
	}
	b.data[id] = struct{}{}
	l.tree.Insert(b)
}

func (l *list) Update(id int, old time.Time, new time.Time) {
	l.mtx.Lock()
	defer l.mtx.Unlock()
	l.delete(id, old)
	l.insert(id, new)
}

func (l *list) Expire(now time.Time) []int {
	l.mtx.Lock()
	defer l.mtx.Unlock()
	deleted := []time.Time{}
	out := []int{}
	elt := l.tree.GetSmallestNode()
	first := elt
	for elt != nil && elt.GetValue().(*bucket).deadline.Before(now) {
		set := elt.GetValue().(*bucket)
		for id := range set.data {
			out = append(out, id)
		}
		deleted = append(deleted, set.deadline)
		elt = l.tree.Next(elt)
		if elt == first {
			break
		}
	}
	for _, t := range deleted {
		l.tree.Delete(&bucket{deadline: t})
	}
	return out
}
