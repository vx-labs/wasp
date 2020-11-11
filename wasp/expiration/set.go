package expiration

import (
	"sync"
	"time"

	"github.com/google/btree"
)

type bucket struct {
	data     map[int]struct{}
	deadline time.Time
}

func (e *bucket) Less(b btree.Item) bool {
	return e.deadline.Before(b.(*bucket).deadline)
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
	tree *btree.BTree
}

func NewList() *list {
	return &list{tree: btree.New(2)}
}

func (l *list) Insert(id int, deadline time.Time) {
	l.mtx.Lock()
	defer l.mtx.Unlock()
	l.insert(id, deadline)
}

func (l *list) Reset() {
	l.tree.Clear(false)
}
func (l *list) Delete(id int, deadline time.Time) {
	l.delete(id, deadline)
}
func (l *list) delete(id int, deadline time.Time) {
	set := l.tree.Get(&bucket{deadline: deadline.Round(time.Second)})
	if set != nil {
		delete(set.(*bucket).data, id)
		if len(set.(*bucket).data) == 0 {
			l.tree.Delete(&bucket{deadline: deadline.Round(time.Second)})
		}
	}
}
func (l *list) insert(id int, deadline time.Time) {
	set := l.tree.Get(&bucket{deadline: deadline.Round(time.Second)})
	if set == nil {
		set = &bucket{data: make(map[int]struct{}), deadline: deadline.Round(time.Second)}
	}
	set.(*bucket).data[id] = struct{}{}
	l.tree.ReplaceOrInsert(set)
}

func (l *list) Update(id int, old time.Time, new time.Time) {
	l.mtx.Lock()
	defer l.mtx.Unlock()
	l.delete(id, old)
	set := l.tree.Get(&bucket{deadline: new.Round(time.Second)})
	if set == nil {
		set = &bucket{data: make(map[int]struct{}), deadline: new.Round(time.Second)}
	}
	set.(*bucket).data[id] = struct{}{}
	l.tree.ReplaceOrInsert(set)
}

func (l *list) Expire(now time.Time) []int {
	l.mtx.Lock()
	defer l.mtx.Unlock()
	deleted := []time.Time{}
	out := []int{}
	l.tree.AscendLessThan(&bucket{deadline: now}, func(i btree.Item) bool {
		set := i.(*bucket)
		for id := range set.data {
			out = append(out, id)
		}
		deleted = append(deleted, set.deadline)
		return true
	})
	for _, t := range deleted {
		l.tree.Delete(&bucket{deadline: t})
	}
	return out
}
