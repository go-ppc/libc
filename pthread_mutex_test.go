// Copyright 2026 The Libc Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !windows && !(linux && (amd64 || arm64 || loong64 || ppc64le || s390x || riscv64 || 386 || arm))

package libc // import "modernc.org/libc"

import (
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"modernc.org/libc/errno"
	"modernc.org/libc/pthread"
)

// Large enough for pthread_mutex_t and pthread_mutexattr_t on all ports using pthread.go.
const testMutexSize = 64

func newTestMutex(t *TLS, typ int32) uintptr {
	p := Xcalloc(t, 1, testMutexSize)
	a := Xcalloc(t, 1, testMutexSize)
	Xpthread_mutexattr_init(t, a)
	Xpthread_mutexattr_settype(t, a, typ)
	Xpthread_mutex_init(t, p, a)
	Xpthread_mutexattr_destroy(t, a)
	Xfree(t, a)
	return p
}

func TestMutexConcurrent(t *testing.T) {
	for _, n := range []int{4, 16, 48} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			tls := NewTLS()
			defer tls.Close()

			normal := newTestMutex(tls, pthread.PTHREAD_MUTEX_NORMAL)
			recursive := newTestMutex(tls, pthread.PTHREAD_MUTEX_RECURSIVE)
			defer Xfree(tls, normal)
			defer Xfree(tls, recursive)

			const iters = 500
			var inNormal, inRecursive atomic.Int32
			var nNormal, nRecursive int // guarded by the pthread mutexes, checked by -race
			var wg sync.WaitGroup
			for i := 0; i < n; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					tls := NewTLS()
					defer tls.Close()

					enter := func(inCS *atomic.Int32, counter *int) {
						if inCS.Add(1) != 1 {
							t.Error("mutual exclusion violated")
						}
						*counter++
						inCS.Add(-1)
					}
					for j := 0; j < iters; j++ {
						if rc := Xpthread_mutex_lock(tls, normal); rc != 0 {
							t.Errorf("lock: %d", rc)
						}
						enter(&inNormal, &nNormal)
						if rc := Xpthread_mutex_unlock(tls, normal); rc != 0 {
							t.Errorf("unlock: %d", rc)
						}

						for Xpthread_mutex_trylock(tls, recursive) == errno.EBUSY {
							runtime.Gosched()
						}
						if rc := Xpthread_mutex_lock(tls, recursive); rc != 0 { // re-entry
							t.Errorf("recursive lock: %d", rc)
						}
						enter(&inRecursive, &nRecursive)
						Xpthread_mutex_unlock(tls, recursive)
						Xpthread_mutex_unlock(tls, recursive)
					}
				}()
			}
			wg.Wait()
			if nNormal != n*iters || nRecursive != n*iters {
				t.Fatalf("counters %d, %d, want %d", nNormal, nRecursive, n*iters)
			}
		})
	}
}

// Statically initialised (zeroed, never init'd) mutexes are created lazily on
// first use; racing first users must all end up on the same Go mutex.
func TestMutexStaticLazyConcurrent(t *testing.T) {
	tls := NewTLS()
	defer tls.Close()

	const n, rounds = 48, 50
	for r := 0; r < rounds; r++ {
		p := Xcalloc(tls, 1, testMutexSize)
		var inCS atomic.Int32
		start := make(chan struct{})
		var wg sync.WaitGroup
		for i := 0; i < n; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				tls := NewTLS()
				defer tls.Close()

				<-start
				Xpthread_mutex_lock(tls, p)
				if inCS.Add(1) != 1 {
					t.Error("mutual exclusion violated on lazily created mutex")
				}
				runtime.Gosched()
				inCS.Add(-1)
				Xpthread_mutex_unlock(tls, p)
			}()
		}
		close(start)
		wg.Wait()
		if mutexCount(p) != 1 {
			t.Fatal("lazily created mutex not registered exactly once")
		}
		Xpthread_mutex_destroy(tls, p)
		Xfree(tls, p)
	}
}

func TestMutexInitDestroyReuse(t *testing.T) {
	tls1 := NewTLS()
	defer tls1.Close()
	tls2 := NewTLS()
	defer tls2.Close()

	p := newTestMutex(tls1, pthread.PTHREAD_MUTEX_RECURSIVE)
	defer Xfree(tls1, p)
	if rc := Xpthread_mutex_lock(tls1, p); rc != 0 {
		t.Fatal(rc)
	}
	if rc := Xpthread_mutex_trylock(tls2, p); rc != errno.EBUSY {
		t.Fatalf("trylock of held mutex: %d", rc)
	}

	// Re-init at the same address must not inherit tls1's ownership.
	Xpthread_mutex_destroy(tls1, p)
	if mutexCount(p) != 0 {
		t.Fatal("destroyed mutex still registered")
	}
	a := Xcalloc(tls1, 1, testMutexSize)
	defer Xfree(tls1, a)
	Xpthread_mutexattr_init(tls1, a)
	Xpthread_mutexattr_settype(tls1, a, pthread.PTHREAD_MUTEX_RECURSIVE)
	Xpthread_mutex_init(tls1, p, a)
	if rc := Xpthread_mutex_trylock(tls2, p); rc != 0 {
		t.Fatalf("trylock of re-initialised mutex: %d", rc)
	}
	Xpthread_mutex_unlock(tls2, p)

	// Destroy then use without init: lazily recreated, unlocked.
	Xpthread_mutex_destroy(tls1, p)
	done := make(chan struct{})
	go func() {
		Xpthread_mutex_lock(tls2, p)
		Xpthread_mutex_unlock(tls2, p)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("lock after destroy blocked")
	}
	Xpthread_mutex_destroy(tls1, p)
}

func BenchmarkMutexLockUnlock(b *testing.B) {
	tls := NewTLS()
	defer tls.Close()
	p := newTestMutex(tls, pthread.PTHREAD_MUTEX_NORMAL)
	defer Xfree(tls, p)
	for b.Loop() {
		Xpthread_mutex_lock(tls, p)
		Xpthread_mutex_unlock(tls, p)
	}
}

func BenchmarkMutexTrylockUnlock(b *testing.B) {
	tls := NewTLS()
	defer tls.Close()
	p := newTestMutex(tls, pthread.PTHREAD_MUTEX_RECURSIVE) // NORMAL trylock always reports EBUSY
	defer Xfree(tls, p)
	for b.Loop() {
		Xpthread_mutex_trylock(tls, p)
		Xpthread_mutex_unlock(tls, p)
	}
}

// Distinct mutexes per goroutine: any slowdown is contention on the mutex table.
func BenchmarkMutexLockUnlockParallel(b *testing.B) {
	b.RunParallel(func(pb *testing.PB) {
		tls := NewTLS()
		defer tls.Close()
		p := newTestMutex(tls, pthread.PTHREAD_MUTEX_NORMAL)
		defer Xfree(tls, p)
		for pb.Next() {
			Xpthread_mutex_lock(tls, p)
			Xpthread_mutex_unlock(tls, p)
		}
	})
}

func mutexCount(p uintptr) int {
	if lookupMutex(p) != nil {
		return 1
	}
	return 0
}
