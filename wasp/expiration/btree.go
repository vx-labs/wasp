package expiration

import (
	"sync"
	"time"

	"github.com/google/btree"
	"github.com/zond/gotomic"
)

type tree struct {
	mtx  sync.RWMutex
	tree *btree.BTree
}

func newTree() List {
	return &tree{tree: btree.New(5)}
}

func (l *tree) Insert(id gotomic.Hashable, deadline time.Time) {
	l.mtx.Lock()
	defer l.mtx.Unlock()
	l.insert(id, deadline.Round(time.Second))
}

func (l *tree) Reset() {
	l.tree = btree.New(5)
}
func (l *tree) Delete(id gotomic.Hashable, deadline time.Time) bool {
	l.mtx.Lock()
	defer l.mtx.Unlock()
	return l.delete(id, deadline)
}
func (l *tree) delete(id gotomic.Hashable, deadline time.Time) bool {
	set := l.tree.Get(&bucket{deadline: deadline.Round(time.Second)})
	if set != nil {
		bucket := set.(*bucket)
		return bucket.delete(id, deadline)
	}
	return false
}
func (l *tree) getBucket(deadline time.Time) *bucket {
	l.mtx.RLock()
	var b *bucket
	set := l.tree.Get(&bucket{deadline: deadline.Round(time.Second)})
	l.mtx.RUnlock()
	if set == nil {
		l.mtx.Lock()
		defer l.mtx.Unlock()
		set = l.tree.Get(&bucket{deadline: deadline.Round(time.Second)})
		if set != nil {
			return set.(*bucket)
		}
		b = &bucket{data: []item{}, deadline: deadline.Round(time.Second)}
		l.tree.ReplaceOrInsert(b)
		return b
	}
	return set.(*bucket)
}
func (l *tree) insert(id gotomic.Hashable, deadline time.Time) {
	var b *bucket
	set := l.tree.Get(&bucket{deadline: deadline.Round(time.Second)})
	if set == nil {
		b = &bucket{data: []item{
			{deadline: deadline, value: id},
		}, deadline: deadline.Round(time.Second)}
		l.tree.ReplaceOrInsert(b)
	} else {
		b = set.(*bucket)
		b.put(id, deadline)
	}
}

func (l *tree) Update(id gotomic.Hashable, old time.Time, new time.Time) {
	b := l.getBucket(old)
	b.delete(id, old)

	b = l.getBucket(new)
	b.put(id, new)
}

func (l *tree) Expire(now time.Time) []interface{} {
	l.mtx.Lock()
	defer l.mtx.Unlock()
	deleted := []time.Time{}
	out := []interface{}{}
	l.tree.AscendLessThan(&bucket{deadline: now.Round(time.Second)}, func(elt btree.Item) bool {
		set := elt.(*bucket)
		for _, v := range set.data {
			out = append(out, v.value)
		}
		deleted = append(deleted, set.deadline)
		return false
	})
	for _, t := range deleted {
		l.tree.Delete(&bucket{deadline: t})
	}
	return out
}
