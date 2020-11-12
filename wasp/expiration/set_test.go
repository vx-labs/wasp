package expiration

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestList(t *testing.T) {
	t.Run("should keep non expired item", func(t *testing.T) {
		l := NewList()
		l.Insert(1, time.Time{})
		expired := l.Expire(time.Time{})
		require.Equal(t, 0, len(expired))
	})
	t.Run("should remove expired items", func(t *testing.T) {
		l := NewList()
		l.Insert(1, time.Time{})
		expired := l.Expire(time.Time{}.Add(1 * time.Minute))
		require.Equal(t, 1, len(expired))
		require.Contains(t, expired, 1)
	})
	t.Run("should allow item renewal", func(t *testing.T) {
		l := NewList()
		l.Insert(1, time.Time{})
		l.Update(1, time.Time{}, time.Time{}.Add(2*time.Minute))
		expired := l.Expire(time.Time{}.Add(1 * time.Minute))
		require.Equal(t, 0, len(expired))
	})
	t.Run("should allow item deletion", func(t *testing.T) {
		l := NewList()
		l.Insert(1, time.Time{})
		l.Delete(1, time.Time{})
		expired := l.Expire(time.Time{}.Add(1 * time.Minute))
		require.Equal(t, 0, len(expired))
	})
	t.Run("should allow set deletion", func(t *testing.T) {
		l := NewList()
		l.Insert(1, time.Time{})
		l.Reset()
		expired := l.Expire(time.Time{}.Add(1 * time.Minute))
		require.Equal(t, 0, len(expired))
	})
}

func BenchmarkSet(b *testing.B) {
	b.Run("update", func(b *testing.B) {
		benchUpdateFunc := func(n int) func(b *testing.B) {
			return func(b *testing.B) {
				l := NewList()
				for i := 0; i < n; i++ {
					l.Insert(i, time.Time{}.Add(time.Duration(i)*time.Second))
				}
				old := time.Time{}.Add(time.Duration(30) * time.Second)
				new := time.Time{}.Add(time.Duration(50) * time.Second)
				for i := 0; i < b.N; i++ {
					l.Update(30, old, new)
					l.Update(30, new, old)
				}
			}
		}
		b.Run("100", benchUpdateFunc(100))
		b.Run("10000", benchUpdateFunc(10000))
		b.Run("1000000", benchUpdateFunc(1000000))
	})
}
