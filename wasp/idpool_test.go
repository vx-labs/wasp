package wasp

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMIDPool(t *testing.T) {
	pool := newMIDPool(1, 500).(*simpleMidPool)
	require.Equal(t, int32(1), pool.Get())
	require.Equal(t, []interval{{from: 1, to: 500}}, pool.intervals)
	require.Equal(t, int32(2), pool.Get())
	require.Equal(t, []interval{{from: 2, to: 500}}, pool.intervals)
	require.Equal(t, int32(3), pool.Get())
	require.Equal(t, []interval{{from: 3, to: 500}}, pool.intervals)
	pool.Put(2)
	require.Equal(t, []interval{{from: 1, to: 2}, {from: 3, to: 500}}, pool.intervals)
	require.Equal(t, int32(2), pool.Get())
	require.Equal(t, []interval{{from: 3, to: 500}}, pool.intervals)
	pool.Put(3)
	require.Equal(t, []interval{{from: 2, to: 500}}, pool.intervals)
	require.Equal(t, int32(3), pool.Get())
	require.Equal(t, int32(4), pool.Get())
	require.Equal(t, int32(5), pool.Get())
	require.Equal(t, int32(6), pool.Get())
	require.Equal(t, []interval{{from: 6, to: 500}}, pool.intervals)
	pool.Put(3)
	require.Equal(t, []interval{{from: 2, to: 3}, {from: 6, to: 500}}, pool.intervals)
	pool.Put(5)
	require.Equal(t, []interval{{from: 2, to: 3}, {from: 4, to: 5}, {from: 6, to: 500}}, pool.intervals)
	pool.Put(4)
	require.Equal(t, []interval{{from: 2, to: 5}, {from: 6, to: 500}}, pool.intervals)
	pool.Put(6)
	require.Equal(t, []interval{{from: 2, to: 500}}, pool.intervals)
	pool.Put(2)
	require.Equal(t, []interval{{from: 1, to: 500}}, pool.intervals)
	pool.Put(400)
	require.Equal(t, []interval{{from: 1, to: 500}}, pool.intervals)
	pool.Put(501)
	require.Equal(t, []interval{{from: 1, to: 500}}, pool.intervals)

	require.Equal(t, int32(2), pool.Get())
	require.Equal(t, []interval{{from: 2, to: 500}}, pool.intervals)
	require.Equal(t, int32(3), pool.Get())
	require.Equal(t, []interval{{from: 3, to: 500}}, pool.intervals)
	require.Equal(t, int32(4), pool.Get())
	require.Equal(t, []interval{{from: 4, to: 500}}, pool.intervals)
	pool.Put(2)
	require.Equal(t, []interval{{from: 1, to: 2}, {from: 4, to: 500}}, pool.intervals)
	pool.Put(3)
	require.Equal(t, []interval{{from: 1, to: 3}, {from: 4, to: 500}}, pool.intervals)
	pool.Put(4)
	require.Equal(t, []interval{{from: 1, to: 500}}, pool.intervals)
}

func BenchmarkMIDPool(b *testing.B) {
	pool := newMIDPool(1, 500).(*simpleMidPool)

	for n := 0; n < b.N; n++ {
		pool.Get()
		pool.Get()
		pool.Get()
		pool.Get()
		pool.Put(3)
		pool.Put(2)
		pool.Put(4)
		pool.Put(1)
	}
}
