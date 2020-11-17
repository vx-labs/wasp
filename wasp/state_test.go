package wasp

import (
	"fmt"
	"testing"

	"github.com/vx-labs/wasp/wasp/sessions"
)

func benchmarkFor(n int) func(b *testing.B) {
	return func(b *testing.B) {
		b.StopTimer()
		s := NewState(1)
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
func benchmarkParallelFor(n int) func(b *testing.B) {
	return func(b *testing.B) {
		b.StopTimer()
		s := NewState(1)
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
		b.Run("100", benchmarkFor(100))
		b.Run("100P", benchmarkParallelFor(100))
		b.Run("10000", benchmarkFor(10000))
		b.Run("10000P", benchmarkParallelFor(10000))
		b.Run("1000000", benchmarkFor(10000))
		b.Run("1000000P", benchmarkParallelFor(10000))
	})
}
