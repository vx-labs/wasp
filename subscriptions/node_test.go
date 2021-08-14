package subscriptions

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTree(t *testing.T) {
	myTree := &tree{
		root: newNode(),
	}
	require.NoError(t, myTree.Upsert([]byte("test/b/c"), func(b []byte) []byte { return []byte("a") }))
	require.Equal(t, 1, len(myTree.root.Children))
	require.Equal(t, []byte("a"), myTree.root.Children["test"].Children["b"].Children["c"].Data)

	found := 0
	myTree.Walk([]byte("test/b/c"), func(b []byte) {
		found++
	})
	require.Equal(t, 1, found)

	require.NoError(t, myTree.Upsert([]byte("test/b/c"), func(b []byte) []byte { return nil }))
	require.Equal(t, 0, len(myTree.root.Children))

	require.NoError(t, myTree.Upsert([]byte("test/b/a"), func(b []byte) []byte { return []byte("a") }))
	require.NoError(t, myTree.Upsert([]byte("test/b/+"), func(b []byte) []byte { return []byte("a") }))
	require.NoError(t, myTree.Upsert([]byte("test/+/a"), func(b []byte) []byte { return []byte("a") }))
	require.NoError(t, myTree.Upsert([]byte("test/#"), func(b []byte) []byte { return []byte("a") }))
	require.NoError(t, myTree.Upsert([]byte("test/c"), func(b []byte) []byte { return []byte("a") }))
	found = 0
	myTree.Walk([]byte("test/b/a"), func(b []byte) {
		require.Equal(t, []byte("a"), b)
		found++
	})
	require.Equal(t, 4, found)

	found = 0
	myTree.Iterate(func(b []byte) {
		require.Equal(t, []byte("a"), b)
		found++
	})
	require.Equal(t, 5, found)
}
