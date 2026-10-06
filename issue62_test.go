// Copyright 2026 The Libc Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Windows has no pthread_mutexattr_settype, so it cannot make the recursive
// mutex these tests need.

//go:build !windows

package libc // import "modernc.org/libc"

import (
	"runtime"
	"sync/atomic"
	"testing"
	"time"
)

func issue62SetInt[T ~int32 | ~int64 | ~int | ~uint32 | ~uint64](p *T, v int64) { *p = T(v) }

// issue62Watchdog runs f and fails the test if f does not return in time,
// which is how a mutex that a condition wait did not release shows up.
func issue62Watchdog(t *testing.T, what string, f func()) {
	t.Helper()
	done := make(chan struct{})
	go func() {
		defer close(done)
		f()
	}()
	select {
	case <-done:
	case <-time.After(30 * time.Second):
		t.Fatalf("%s: no progress after 30 seconds, deadlock", what)
	}
}

func issue62Mutex(tls *TLS, typ int32) (m, c uintptr, free func()) {
	attr := Xcalloc(tls, 1, 64)
	m = Xcalloc(tls, 1, 64)
	c = Xcalloc(tls, 1, 64)
	Xpthread_mutexattr_init(tls, attr)
	Xpthread_mutexattr_settype(tls, attr, typ)
	Xpthread_mutex_init(tls, m, attr)
	Xpthread_mutexattr_destroy(tls, attr)
	Xpthread_cond_init(tls, c, 0)
	return m, c, func() {
		Xpthread_cond_destroy(tls, c)
		Xpthread_mutex_destroy(tls, m)
		Xfree(tls, c)
		Xfree(tls, m)
		Xfree(tls, attr)
	}
}

var issue62Types = []struct {
	name string
	typ  int32
}{
	{"normal", issue62Normal},
	{"recursive", issue62Recursive},
}

// TestIssue62CondWait checks that pthread_cond_wait releases the mutex, so
// that another thread can take it and signal, and holds it again on return.
func TestIssue62CondWait(t *testing.T) {
	for _, tt := range issue62Types {
		t.Run(tt.name, func(t *testing.T) {
			issue62Watchdog(t, "pthread_cond_wait", func() {
				tls := NewTLS()
				defer tls.Close()

				m, c, free := issue62Mutex(tls, tt.typ)
				defer free()

				var ready atomic.Bool
				done := make(chan struct{})
				Xpthread_mutex_lock(tls, m)
				go func() {
					defer close(done)

					tls := NewTLS()
					defer tls.Close()

					// This succeeds only once the wait below has released m.
					Xpthread_mutex_lock(tls, m)
					ready.Store(true)
					Xpthread_cond_signal(tls, c)
					Xpthread_mutex_unlock(tls, m)
				}()
				for !ready.Load() {
					if r := Xpthread_cond_wait(tls, c, m); r != 0 {
						t.Errorf("pthread_cond_wait: %d", r)
						break
					}
				}
				Xpthread_mutex_unlock(tls, m)
				<-done
			})
		})
	}
}

// TestIssue62CondTimedwait checks that pthread_cond_timedwait returns
// ETIMEDOUT with the mutex held again at the depth it had before, including
// a recursive mutex locked twice, of which the wait releases one level.
func TestIssue62CondTimedwait(t *testing.T) {
	for _, tt := range []struct {
		name  string
		typ   int32
		depth int
	}{
		{"normal", issue62Normal, 1},
		{"recursive", issue62Recursive, 1},
		{"recursive_twice", issue62Recursive, 2},
	} {
		t.Run(tt.name, func(t *testing.T) {
			issue62Watchdog(t, "pthread_cond_timedwait", func() {
				tls := NewTLS()
				defer tls.Close()

				m, c, free := issue62Mutex(tls, tt.typ)
				defer free()

				ts := Xcalloc(tls, 1, 64)
				defer Xfree(tls, ts)

				for i := 0; i < tt.depth; i++ {
					Xpthread_mutex_lock(tls, m)
				}
				issue62Deadline(ts, 50*time.Millisecond)
				if r := Xpthread_cond_timedwait(tls, c, m, ts); r != issue62ETIMEDOUT {
					t.Errorf("pthread_cond_timedwait: %d, want ETIMEDOUT (%d)", r, issue62ETIMEDOUT)
				}

				// Another thread must get the mutex only after the last unlock.
				acquired := make(chan struct{})
				done := make(chan struct{})
				go func() {
					defer close(done)

					tls := NewTLS()
					defer tls.Close()

					Xpthread_mutex_lock(tls, m)
					close(acquired)
					Xpthread_mutex_unlock(tls, m)
				}()
				for i := 0; i < tt.depth; i++ {
					select {
					case <-acquired:
						t.Errorf("another thread got the mutex with %d of %d levels still locked", tt.depth-i, tt.depth)
					case <-time.After(20 * time.Millisecond):
					}
					Xpthread_mutex_unlock(tls, m)
				}
				<-done
			})
		})
	}
}

// TestIssue62CondTimedwaitExpired checks that a pthread_cond_timedwait whose
// deadline has already passed leaves nothing behind that takes the next
// pthread_cond_signal away from a thread that is really waiting.
func TestIssue62CondTimedwaitExpired(t *testing.T) {
	issue62Watchdog(t, "pthread_cond_signal after an expired pthread_cond_timedwait", func() {
		tls := NewTLS()
		defer tls.Close()

		m, c, free := issue62Mutex(tls, issue62Normal)
		defer free()

		ts := Xcalloc(tls, 1, 64)
		defer Xfree(tls, ts)

		for i := 0; i < 20; i++ {
			Xpthread_mutex_lock(tls, m)
			issue62Deadline(ts, -time.Second)
			if r := Xpthread_cond_timedwait(tls, c, m, ts); r != issue62ETIMEDOUT {
				t.Errorf("pthread_cond_timedwait: %d, want ETIMEDOUT (%d)", r, issue62ETIMEDOUT)
			}

			var started, ready atomic.Bool
			done := make(chan struct{})
			go func() {
				defer close(done)

				tls := NewTLS()
				defer tls.Close()

				Xpthread_mutex_lock(tls, m)
				started.Store(true)
				for !ready.Load() {
					Xpthread_cond_wait(tls, c, m)
				}
				Xpthread_mutex_unlock(tls, m)
			}()
			Xpthread_mutex_unlock(tls, m)

			// The waiter holds m from setting started until its wait releases
			// it, so seeing started under m means it is waiting now.
			for {
				Xpthread_mutex_lock(tls, m)
				if started.Load() {
					break
				}

				Xpthread_mutex_unlock(tls, m)
				runtime.Gosched()
			}
			ready.Store(true)
			Xpthread_cond_signal(tls, c)
			Xpthread_mutex_unlock(tls, m)
			<-done
		}
	})
}
