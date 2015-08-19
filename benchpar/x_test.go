package x

import (
	"sync"
	"sync/atomic"
	"testing"
)

var (
	mu sync.Mutex
	n  int64
)

func BenchmarkLockUnlock(b *testing.B) {
	bench(b, func() {
		mu.Lock()
		n++
		mu.Unlock()
	})
}

func BenchmarkLockDeferUnlock(b *testing.B) {
	bench(b, func() {
		mu.Lock()
		defer mu.Unlock()
		n++
	})
}

func BenchmarkDeferUnlockLock(b *testing.B) {
	bench(b, func() {
		defer mu.Unlock()
		mu.Lock()
		n++
	})
}

func BenchmarkAtomic(b *testing.B) {
	bench(b, func() {
		atomic.AddInt64(&n, 1)
	})
}

func bench(b *testing.B, fn func()) {
	const parallel = true
	if parallel {
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				fn()
			}
		})
	} else {
		for i := 0; i < b.N; i++ {
			fn()
		}
	}
}
