package sessions

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPrefixMountPoint(t *testing.T) {
	require.Equal(t, []byte("_default/test"), prefixMountPoint("_default", []byte("test")))
}
func TestTrimMountPoint(t *testing.T) {
	require.Equal(t, []byte("test"), trimMountPoint("_default", []byte("_default/test")))
}
func BenchmarkPrefixMountPoint(b *testing.B) {
	for i := 0; i < b.N; i++ {
		prefixMountPoint("_default", []byte("test"))
	}
}
func BenchmarkTrimMountPoint(b *testing.B) {
	for i := 0; i < b.N; i++ {
		trimMountPoint("_default", []byte("_default/test"))
	}
}
