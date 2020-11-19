package expiration

import (
	"testing"
)

func TestPQList(t *testing.T) {
	testSuite(newPQList)(t)
}

func BenchmarkPQList(b *testing.B) {
	benchSuite(newPQList)(b)
}
