package expiration

import (
	"sync"
	"time"

	"github.com/MauriceGit/skiplist"
)

type list struct {
	mtx  sync.RWMutex
	tree skiplist.SkipList
}

func newSkipList() List {
	return &list{tree: skiplist.New()}
}

func (l *list) Insert(id interface{}, deadline time.Time) {
	l.mtx.Lock()
	defer l.mtx.Unlock()
	l.insert(id, deadline.Round(time.Second))
}

func (l *list) Reset() {
	l.tree = skiplist.New()
}
func (l *list) Delete(id interface{}, deadline time.Time) bool {
	l.mtx.Lock()
	defer l.mtx.Unlock()
	return l.delete(id, deadline)
}
func (l *list) delete(id interface{}, deadline time.Time) bool {
	set, ok := l.tree.Find(&bucket{deadline: deadline.Round(time.Second)})
	if ok {
		bucket := set.GetValue().(*bucket)
		return bucket.delete(id, deadline)
	}
	return ok
}
func (l *list) getBucket(deadline time.Time) *bucket {
	l.mtx.RLock()
	var b *bucket
	set, ok := l.tree.Find(&bucket{deadline: deadline.Round(time.Second)})
	l.mtx.RUnlock()
	if !ok {
		l.mtx.Lock()
		defer l.mtx.Unlock()
		set, ok = l.tree.Find(&bucket{deadline: deadline.Round(time.Second)})
		if ok {
			return set.GetValue().(*bucket)
		}
		b = &bucket{data: []item{}, deadline: deadline.Round(time.Second)}
		l.tree.Insert(b)
		return b
	}
	return set.GetValue().(*bucket)
}
func (l *list) insert(id interface{}, deadline time.Time) {
	var b *bucket
	set, ok := l.tree.Find(&bucket{deadline: deadline.Round(time.Second)})
	if !ok {
		b = &bucket{data: []item{
			{deadline: deadline, value: id},
		}, deadline: deadline.Round(time.Second)}
		l.tree.Insert(b)
	} else {
		b = set.GetValue().(*bucket)
		b.put(id, deadline)
	}
}

func (l *list) Update(id interface{}, old time.Time, new time.Time) {
	b := l.getBucket(old)
	b.delete(id, old)

	b = l.getBucket(new)
	b.put(id, new)
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
		for _, v := range set.data {
			out = append(out, v.value)
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
