package topics

import (
	"bytes"
	fmt "fmt"
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
		old, err := tree.Insert([]byte("devices/cars/a"), []byte("a"))
		require.False(t, old)
		require.NoError(t, err)
		old,err = tree.Insert([]byte("devices/cars/b"), []byte("b"))
		require.False(t, old)
		require.NoError(t, err)
		old,err = tree.Insert([]byte("devices/bicycle/c"), []byte("c"))
		require.False(t, old)
		require.NoError(t, err)
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

func BenchmarkTree(b *testing.B) {
	b.Run("insert", func(b *testing.B) {
		tree := NewTree()
		for i := 0; i < b.N; i++ {
			tree.Insert([]byte("test/1"), []byte("test"))
		}
	})
	b.Run("get", func(b *testing.B) {
		benchmarkOnSize(50,
			func(tree Store) { tree.Match([]byte("test/40"), &[][]byte{}) })(b)
		benchmarkOnSize(500,
			func(tree Store) { tree.Match([]byte("test/40"), &[][]byte{}) })(b)
		benchmarkOnSize(5000,
			func(tree Store) { tree.Match([]byte("test/40"), &[][]byte{}) })(b)
	})
	b.Run("wildcard", func(b *testing.B) {
		benchmarkOnSize(50,
			func(tree Store) { tree.Match([]byte("test/+"), &[][]byte{}) })(b)
		benchmarkOnSize(500,
			func(tree Store) { tree.Match([]byte("test/+"), &[][]byte{}) })(b)
		benchmarkOnSize(5000,
			func(tree Store) { tree.Match([]byte("test/+"), &[][]byte{}) })(b)
	})
}

func benchmarkOnSize(size int, f func(Store)) func(b *testing.B) {
	return func(b *testing.B) {
		tree := NewTree()
		for i := 0; i < size; i++ {
			tree.Insert([]byte(fmt.Sprintf("test/%d", i)), []byte("test"))
		}
		b.Run(fmt.Sprintf("%d", size), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				f(tree)
			}
		})
	}
}
