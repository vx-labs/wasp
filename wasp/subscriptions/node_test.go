package subscriptions

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTree_Insert_Remove(t *testing.T) {
	myTree := &tree{
		root: newNode(),
	}
	require.NoError(t, myTree.Insert(1, []byte("test/a"), 1, "test"))
	require.NoError(t, myTree.Insert(2, []byte("test/b"), 1, "test2"))
	require.NoError(t, myTree.Insert(3, []byte("test/b/c"), 1, "test3"))
	require.NotNil(t, myTree.root.Children["test"])
	require.NotNil(t, myTree.root.Children["test"].Children["a"])
	require.NotNil(t, myTree.root.Children["test"].Children["b"])
	require.Equal(t, "test", myTree.root.Children["test"].Children["a"].Recipients[0])
	require.Equal(t, "test2", myTree.root.Children["test"].Children["b"].Recipients[0])
	require.Equal(t, "test3", myTree.root.Children["test"].Children["b"].Children["c"].Recipients[0])
	require.NoError(t, myTree.Remove([]byte("test/b/c"), "test3"))
	require.Nil(t, myTree.root.Children["test"].Children["b"].Children["c"])
}
func TestTree_Match(t *testing.T) {
	myTree := &tree{
		root: newNode(),
	}
	require.NoError(t, myTree.Insert(1, []byte("test/a"), 1, "test"))
	require.NoError(t, myTree.Insert(1, []byte("test/+"), 1, "test2"))
	require.NoError(t, myTree.Insert(2, []byte("test/b/c"), 1, "test3"))
	recipientIds := []string{}
	recipientQos := []int32{}
	recipientPeers := []uint64{}
	require.NoError(t, myTree.Match([]byte("test/b/c"), 0, &recipientPeers, &recipientIds, &recipientQos))
	require.Equal(t, "test3", recipientIds[0])
	require.NoError(t, myTree.Match([]byte("test/a"), 0, &recipientPeers, &recipientIds, &recipientQos))
	require.Equal(t, "test", recipientIds[0])
	require.Equal(t, "test2", recipientIds[1])
	require.Equal(t, uint64(1), recipientPeers[1])
}
