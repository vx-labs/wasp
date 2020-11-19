package expiration

import (
	"sync"
	"time"

	"github.com/MauriceGit/skiplist"
	"github.com/zond/gotomic"
)

type list struct {
	mtx  sync.Mutex
	tree skiplist.SkipList
}

func NewList() List {
	return &list{tree: skiplist.New()}
}

func (l *list) Insert(id gotomic.Hashable, deadline time.Time) {
	l.mtx.Lock()
	defer l.mtx.Unlock()
	l.insert(id, deadline.Round(time.Second))
}

func (l *list) Reset() {
	l.tree = skiplist.New()
}
func (l *list) Delete(id gotomic.Hashable, deadline time.Time) bool {
	l.mtx.Lock()
	defer l.mtx.Unlock()
	return l.delete(id, deadline)
}
func (l *list) delete(id gotomic.Hashable, deadline time.Time) bool {
	set, ok := l.tree.Find(&bucket{deadline: deadline.Round(time.Second)})
	if ok {
		bucket := set.GetValue().(*bucket)
		bucket.data.Delete(id)
	}
	return ok
}
func (l *list) insert(id gotomic.Hashable, deadline time.Time) {
	var b *bucket
	set, ok := l.tree.Find(&bucket{deadline: deadline.Round(time.Second)})
	if !ok {
		b = &bucket{data: gotomic.NewHash(), deadline: deadline.Round(time.Second)}
		l.tree.Insert(b)
	} else {
		b = set.GetValue().(*bucket)
	}
	b.data.PutIfMissing(id, nil)
}

func (l *list) Update(id gotomic.Hashable, old time.Time, new time.Time) {
	l.mtx.Lock()
	defer l.mtx.Unlock()
	l.delete(id, old)
	l.insert(id, new)
}

func (l *list) Expire(now time.Time) []interface{} {
	l.mtx.Lock()
	defer l.mtx.Unlock()
	deleted := []time.Time{}
	out := []interface{}{}
	elt := l.tree.GetSmallestNode()
	first := elt
	for elt != nil && elt.GetValue().(*bucket).deadline.Before(now) {
		set := elt.GetValue().(*bucket)
		for id := range set.data.ToMap() {
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
