package wasp

import (
	"fmt"
	"testing"

	"github.com/vx-labs/wasp/wasp/sessions"
	"github.com/zond/gotomic"
)

func benchmarkFor(st func() LocalState, n int) func(b *testing.B) {
	return func(b *testing.B) {
		b.StopTimer()
		s := st()
		for i := 0; i < n; i++ {
			s.Create(fmt.Sprintf("%d", i), &sessions.Session{})
		}
		b.ResetTimer()
		b.StartTimer()
		for i := 0; i < b.N; i++ {
			s.Get("40")
		}
	}
}
func benchmarkParallelFor(st func() LocalState, n int) func(b *testing.B) {
	return func(b *testing.B) {
		b.StopTimer()
		s := st()
		for i := 0; i < n; i++ {
			s.Create(fmt.Sprintf("%d", i), &sessions.Session{})
		}
		b.ResetTimer()
		b.StartTimer()
		b.RunParallel(func(p *testing.PB) {
			for p.Next() {
				s.Get("40")
			}
		})
	}
}
func BenchmarkState(b *testing.B) {
	b.Run("get", func(b *testing.B) {
		b.Run("hashmap", func(b *testing.B) {
			prov := func() LocalState { return &hashmapState{sessions: gotomic.NewHash()} }
			b.Run("100", benchmarkFor(prov, 100))
			b.Run("100P", benchmarkParallelFor(prov, 100))
			b.Run("10000", benchmarkFor(prov, 10000))
			b.Run("10000P", benchmarkParallelFor(prov, 10000))
			b.Run("1000000", benchmarkFor(prov, 10000))
			b.Run("1000000P", benchmarkParallelFor(prov, 10000))
		})
		b.Run("lockedMap", func(b *testing.B) {
			prov := func() LocalState { return &lockedMapState{sessions: map[string]*sessions.Session{}} }
			b.Run("100", benchmarkFor(prov, 100))
			b.Run("100P", benchmarkParallelFor(prov, 100))
			b.Run("10000", benchmarkFor(prov, 10000))
			b.Run("10000P", benchmarkParallelFor(prov, 10000))
			b.Run("1000000", benchmarkFor(prov, 10000))
			b.Run("1000000P", benchmarkParallelFor(prov, 10000))
		})
	})
}
