package expiration

import (
	"time"

	"github.com/google/btree"
	"github.com/zond/gotomic"
)

type bucket struct {
	data     *gotomic.Hash
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
