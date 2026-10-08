// Copyright 2026 The Libc Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build linux && (amd64 || arm64 || loong64 || ppc64le || s390x || riscv64 || 386 || arm)

package libc // import "modernc.org/libc"

import (
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestIssue63RecursiveTrylock checks pthread_mutex_trylock on a recursive mutex,
// which used to panic: the owner gets it again, another thread gets EBUSY until
// the last unlock.
func TestIssue63RecursiveTrylock(t *testing.T) {
	a, b := NewTLS(), NewTLS()
	defer a.Close()
	defer b.Close()

	m, _, free := issue62Mutex(a, PTHREAD_MUTEX_RECURSIVE)
	defer free()

	for i, step := range []struct {
		tls  *TLS
		op   func(*TLS, uintptr) int32
		name string
		want int32
	}{
		{a, Xpthread_mutex_trylock, "a trylock, free", 0},
		{a, Xpthread_mutex_trylock, "a trylock, owned by a", 0},
		{b, Xpthread_mutex_trylock, "b trylock, owned by a", EBUSY},
		{a, Xpthread_mutex_lock, "a lock, owned by a", 0},
		{b, Xpthread_mutex_unlock, "b unlock, owned by a", EPERM},
		{a, Xpthread_mutex_unlock, "a unlock 1 of 3", 0},
		{a, Xpthread_mutex_unlock, "a unlock 2 of 3", 0},
		{b, Xpthread_mutex_trylock, "b trylock, a holds one level", EBUSY},
		{a, Xpthread_mutex_unlock, "a unlock 3 of 3", 0},
		{b, Xpthread_mutex_trylock, "b trylock, free", 0},
		{a, Xpthread_mutex_trylock, "a trylock, owned by b", EBUSY},
		{b, Xpthread_mutex_unlock, "b unlock", 0},
	} {
		if got := step.op(step.tls, m); got != step.want {
			t.Fatalf("step %d, %s: got %d, want %d", i, step.name, got, step.want)
		}
	}
}

// TestIssue63Errorcheck checks an error-checking mutex, every use of which used
// to panic.
func TestIssue63Errorcheck(t *testing.T) {
	a, b := NewTLS(), NewTLS()
	defer a.Close()
	defer b.Close()

	m, _, free := issue62Mutex(a, PTHREAD_MUTEX_ERRORCHECK)
	defer free()

	for i, step := range []struct {
		tls  *TLS
		op   func(*TLS, uintptr) int32
		name string
		want int32
	}{
		{a, Xpthread_mutex_unlock, "a unlock, free", EPERM},
		{a, Xpthread_mutex_lock, "a lock, free", 0},
		{a, Xpthread_mutex_lock, "a lock, owned by a", EDEADLK},
		{a, Xpthread_mutex_trylock, "a trylock, owned by a", EBUSY},
		{b, Xpthread_mutex_trylock, "b trylock, owned by a", EBUSY},
		{b, Xpthread_mutex_unlock, "b unlock, owned by a", EPERM},
		{a, Xpthread_mutex_unlock, "a unlock", 0},
		{a, Xpthread_mutex_unlock, "a unlock again", EPERM},
		{b, Xpthread_mutex_trylock, "b trylock, free", 0},
		{b, Xpthread_mutex_unlock, "b unlock", 0},
		{b, Xpthread_mutex_lock, "b lock, free", 0},
		{b, Xpthread_mutex_unlock, "b unlock", 0},
	} {
		if got := step.op(step.tls, m); got != step.want {
			t.Fatalf("step %d, %s: got %d, want %d", i, step.name, got, step.want)
		}
	}
}

// TestIssue63ErrorcheckCond checks condition waits on an error-checking mutex:
// a timed wait by the owner times out holding the mutex, a wait by a thread
// that does not hold it reports EPERM, and a wait releases it for a signaler.
func TestIssue63ErrorcheckCond(t *testing.T) {
	issue62Watchdog(t, "condition wait on an error-checking mutex", func() {
		a, b := NewTLS(), NewTLS()
		defer a.Close()
		defer b.Close()

		m, c, free := issue62Mutex(a, PTHREAD_MUTEX_ERRORCHECK)
		defer free()

		ts := Xcalloc(a, 1, 64)
		defer Xfree(a, ts)

		Xpthread_mutex_lock(a, m)
		issue62Deadline(ts, 20*time.Millisecond)
		if got := Xpthread_cond_timedwait(a, c, m, ts); got != ETIMEDOUT {
			t.Errorf("owner timedwait: got %d, want ETIMEDOUT", got)
		}
		if got := Xpthread_mutex_lock(a, m); got != EDEADLK {
			t.Errorf("lock after the wait: got %d, want EDEADLK, the wait must leave it held", got)
		}
		issue62Deadline(ts, 20*time.Millisecond)
		if got := Xpthread_cond_timedwait(b, c, m, ts); got != EPERM {
			t.Errorf("timedwait by a thread that does not hold it: got %d, want EPERM", got)
		}

		var ready atomic.Bool
		done := make(chan struct{})
		go func() {
			defer close(done)

			tls := NewTLS()
			defer tls.Close()

			Xpthread_mutex_lock(tls, m) // only once the wait below released it
			ready.Store(true)
			Xpthread_cond_signal(tls, c)
			Xpthread_mutex_unlock(tls, m)
		}()
		for !ready.Load() {
			if got := Xpthread_cond_wait(a, c, m); got != 0 {
				t.Errorf("cond_wait: %d", got)
				break
			}
		}
		if got := Xpthread_mutex_unlock(a, m); got != 0 {
			t.Errorf("unlock after the wait: %d", got)
		}
		<-done
	})
}

// TestIssue63Stress mixes lock and trylock on a recursive and an error-checking
// mutex from many threads and checks mutual exclusion and the final counts.
func TestIssue63Stress(t *testing.T) {
	issue62Watchdog(t, "trylock stress", func() {
		owner := NewTLS()
		defer owner.Close()

		rec, _, freeRec := issue62Mutex(owner, PTHREAD_MUTEX_RECURSIVE)
		defer freeRec()

		ec, _, freeEc := issue62Mutex(owner, PTHREAD_MUTEX_ERRORCHECK)
		defer freeEc()

		// 16 threads contend enough. Many more only make the run slow under
		// -race, where every hand-off wakes all waiting threads.
		const n, iters = 16, 1000
		var inRec, inEc atomic.Int32
		var nRec, nEc int // guarded by the mutexes
		var wg sync.WaitGroup
		for g := 0; g < n; g++ {
			wg.Add(1)
			go func(g int) {
				defer wg.Done()

				tls := NewTLS()
				defer tls.Close()

				for i := 0; i < iters; i++ {
					if (g+i)%2 == 0 {
						Xpthread_mutex_lock(tls, rec)
					} else {
						for Xpthread_mutex_trylock(tls, rec) != 0 {
							runtime.Gosched()
						}
					}
					if got := Xpthread_mutex_trylock(tls, rec); got != 0 { // re-entry
						t.Errorf("recursive re-entry: %d", got)
					}
					if inRec.Add(1) != 1 {
						t.Error("two holders of the recursive mutex")
					}
					nRec++
					inRec.Add(-1)
					Xpthread_mutex_unlock(tls, rec)
					Xpthread_mutex_unlock(tls, rec)

					if (g+i)%2 == 0 {
						Xpthread_mutex_lock(tls, ec)
					} else {
						for Xpthread_mutex_trylock(tls, ec) != 0 {
							runtime.Gosched()
						}
					}
					if inEc.Add(1) != 1 {
						t.Error("two holders of the error-checking mutex")
					}
					nEc++
					inEc.Add(-1)
					if got := Xpthread_mutex_unlock(tls, ec); got != 0 {
						t.Errorf("errorcheck unlock: %d", got)
					}
				}
			}(g)
		}
		wg.Wait()
		if nRec != n*iters || nEc != n*iters {
			t.Errorf("counts %d and %d, want %d", nRec, nEc, n*iters)
		}
	})
}
