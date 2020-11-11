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
