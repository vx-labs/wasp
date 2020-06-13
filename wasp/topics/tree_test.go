package topics

import (
	"bytes"
	"sort"
	"testing"

	"github.com/stretchr/testify/require"
)

func sortResults(out [][]byte) {
	sort.Slice(out, func(i, j int) bool {
		return bytes.Compare(out[i], out[j]) == -1
	})
}

func TestTree(t *testing.T) {
	tree := NewTree()
	t.Run("insert", func(t *testing.T) {
		require.NoError(t, tree.Insert([]byte("devices/cars/a"), []byte("a")))
		require.NoError(t, tree.Insert([]byte("devices/cars/b"), []byte("b")))
		require.NoError(t, tree.Insert([]byte("devices/bicycle/c"), []byte("c")))
	})
	t.Run("match", func(t *testing.T) {
		out := make([][]byte, 0)
		require.NoError(t, tree.Match([]byte("devices/cars/a"), &out))
		sortResults(out)
		require.Equal(t, 1, len(out))
		require.Equal(t, []byte("a"), out[0])

		out = make([][]byte, 0)
		require.NoError(t, tree.Match([]byte("devices/cars/+"), &out))
		sortResults(out)
		require.Equal(t, 2, len(out))
		require.Equal(t, []byte("a"), out[0])
		require.Equal(t, []byte("b"), out[1])

		out = make([][]byte, 0)
		require.NoError(t, tree.Match([]byte("devices/#"), &out))
		sortResults(out)
		require.Equal(t, 3, len(out))
		require.Equal(t, []byte("a"), out[0])
		require.Equal(t, []byte("b"), out[1])
		require.Equal(t, []byte("c"), out[2])
	})
	t.Run("remove", func(t *testing.T) {
		require.NoError(t, tree.Remove([]byte("devices/cars/a")))

		out := make([][]byte, 0)
		require.NoError(t, tree.Match([]byte("devices/cars/a"), &out))
		require.Equal(t, 0, len(out))
	})

}
