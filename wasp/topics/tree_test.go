package topics

import (
	"bytes"
	"sort"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/vx-labs/mqtt-protocol/packet"
)

func sortPublishes(out []*packet.Publish) {
	sort.Slice(out, func(i, j int) bool {
		return bytes.Compare(out[i].Topic, out[j].Topic) == -1
	})
}

func TestTree(t *testing.T) {
	tree := NewTree()
	t.Run("insert", func(t *testing.T) {
		require.NoError(t, tree.Insert(&packet.Publish{
			Header:  &packet.Header{},
			Topic:   []byte("devices/cars/a"),
			Payload: []byte("test"),
		}))
		require.NoError(t, tree.Insert(&packet.Publish{
			Header:  &packet.Header{},
			Topic:   []byte("devices/cars/b"),
			Payload: []byte("test"),
		}))
		require.NoError(t, tree.Insert(&packet.Publish{
			Header:  &packet.Header{},
			Topic:   []byte("devices/bicycle/c"),
			Payload: []byte("test"),
		}))
	})
	t.Run("match", func(t *testing.T) {
		out := make([]*packet.Publish, 0)
		require.NoError(t, tree.Match([]byte("devices/cars/a"), &out))
		sortPublishes(out)
		require.Equal(t, 1, len(out))
		require.Equal(t, []byte("test"), out[0].Payload)
		require.Equal(t, []byte("devices/cars/a"), out[0].Topic)

		out = make([]*packet.Publish, 0)
		require.NoError(t, tree.Match([]byte("devices/cars/+"), &out))
		sortPublishes(out)
		require.Equal(t, 2, len(out))
		require.Equal(t, []byte("devices/cars/a"), out[0].Topic)
		require.Equal(t, []byte("devices/cars/b"), out[1].Topic)

		out = make([]*packet.Publish, 0)
		require.NoError(t, tree.Match([]byte("devices/#"), &out))
		sortPublishes(out)
		require.Equal(t, 3, len(out))
		require.Equal(t, []byte("devices/bicycle/c"), out[0].Topic)
		require.Equal(t, []byte("devices/cars/a"), out[1].Topic)
		require.Equal(t, []byte("devices/cars/b"), out[2].Topic)
	})
	t.Run("remove", func(t *testing.T) {
		require.NoError(t, tree.Remove([]byte("devices/cars/a")))

		out := make([]*packet.Publish, 0)
		require.NoError(t, tree.Match([]byte("devices/cars/a"), &out))
		require.Equal(t, 0, len(out))
	})

}
