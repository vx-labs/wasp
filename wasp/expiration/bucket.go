package expiration

import (
	"time"

	"github.com/zond/gotomic"
)

type bucket struct {
	index    int
	data     *gotomic.Hash
	deadline time.Time
}

func (e *bucket) ExtractKey() float64 {
	return float64(e.deadline.Unix())
}
func (e *bucket) String() string {
	return e.deadline.String()
}

type mapBucket struct {
	index    int
	data     map[interface{}]struct{}
	deadline time.Time
}
