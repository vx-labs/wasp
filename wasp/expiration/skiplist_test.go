package expiration

import (
	"testing"
)

func TestList(t *testing.T) {
	testSuite(newSkipList)(t)
}

func BenchmarkList(b *testing.B) {
	benchSuite(newSkipList)(b)
}
