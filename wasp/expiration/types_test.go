package expiration

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/zond/gotomic"
)

type key int

func (k key) HashCode() uint32 {
	return uint32(k)
}
func (k key) Equals(v gotomic.Thing) bool {
	return k == v.(key)
}

func benchSuite(newList func() List) func(b *testing.B) {
	return func(b *testing.B) {
		b.Run("update", func(b *testing.B) {
			benchUpdateFunc := func(n int) func(b *testing.B) {
				return func(b *testing.B) {
					b.StopTimer()
					l := newList()
					for i := 0; i < n; i++ {
						l.Insert(key(i), time.Time{}.Add(time.Duration(i)*time.Second))
					}
					old := time.Time{}.Add(time.Duration(30) * time.Second)
					new := time.Time{}.Add(time.Duration(50) * time.Second)
					b.ResetTimer()
					b.StartTimer()
					for i := 0; i < b.N; i++ {
						l.Update(key(30), old, new)
						l.Update(key(30), new, old)
					}
				}
			}
			b.Run("100", benchUpdateFunc(100))
			b.Run("10000", benchUpdateFunc(10000))
			b.Run("1000000", benchUpdateFunc(1000000))
		})
	}
}
func testSuite(newList func() List) func(t *testing.T) {
	return func(t *testing.T) {
		t.Run("should keep non expired item", func(t *testing.T) {
			l := newList()
			l.Insert(key(1), time.Time{})
			expired := l.Expire(time.Time{})
			require.Equal(t, 0, len(expired))
		})
		t.Run("should remove expired items", func(t *testing.T) {
			l := newList()
			l.Insert(key(1), time.Time{})
			expired := l.Expire(time.Time{}.Add(1 * time.Minute))
			require.Equal(t, 1, len(expired))
			require.Contains(t, expired, key(1))
		})
		t.Run("should allow item renewal", func(t *testing.T) {
			l := newList()
			l.Insert(key(1), time.Time{})
			l.Update(key(1), time.Time{}, time.Time{}.Add(2*time.Minute))
			expired := l.Expire(time.Time{}.Add(1 * time.Minute))
			require.Equal(t, 0, len(expired))
		})
		t.Run("should allow item deletion", func(t *testing.T) {
			l := newList()
			l.Insert(key(1), time.Time{})
			l.Delete(key(1), time.Time{})
			expired := l.Expire(time.Time{}.Add(1 * time.Minute))
			require.Equal(t, 0, len(expired))
		})
		t.Run("should allow set deletion", func(t *testing.T) {
			l := newList()
			l.Insert(key(1), time.Time{})
			l.Reset()
			expired := l.Expire(time.Time{}.Add(1 * time.Minute))
			require.Equal(t, 0, len(expired))
		})

	}
}
