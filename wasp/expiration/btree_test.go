package expiration

import (
	"testing"
)

func TestTree(t *testing.T) {
	testSuite(newTree)(t)
}

func BenchmarkTree(b *testing.B) {
	benchSuite(newTree)(b)
}
