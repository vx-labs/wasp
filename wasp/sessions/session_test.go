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

// func TestSessionSize(t *testing.T) {
// 	var s Session
// 	sessionSize := unsafe.Sizeof(s)
// 	require.Equal(t, 10, int(sessionSize))
// }

func TestSessionRemoveTopic(t *testing.T) {
	var s Session
	s.AddTopic([]byte("a"))
	s.AddTopic([]byte("b"))
	s.AddTopic([]byte("c"))
	s.RemoveTopic([]byte("b"))
	require.Equal(t, 2, len(s.GetTopics()))
	s.RemoveTopic([]byte("a"))
	require.Equal(t, 1, len(s.GetTopics()))
	s.RemoveTopic([]byte("c"))
	require.Equal(t, 0, len(s.GetTopics()))
	s.AddTopic([]byte("c"))
	s.AddTopic([]byte("c"))
	require.Equal(t, 1, len(s.GetTopics()))
	s.RemoveTopic([]byte("c"))
	require.Equal(t, 0, len(s.GetTopics()))
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
