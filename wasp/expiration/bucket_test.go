package expiration

import (
	"testing"
	"time"
)

func BenchmarkBucket(b *testing.B) {
	elt := &bucket{
		deadline: time.Time{}.Add(time.Minute),
		data:     []item{},
	}
	t := time.Time{}.Add(time.Minute)
	for i := 0; i < b.N; i++ {
		elt.put(i, t)
		elt.delete(i, t)
	}
}
