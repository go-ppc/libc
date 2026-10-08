// Copyright 2026 The Libc Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !(linux && (amd64 || arm64 || loong64 || ppc64le || s390x || riscv64 || 386 || arm))

package libc // import "modernc.org/libc"

import (
	"testing"

	"modernc.org/libc/errno"
)

func TestMutexTrylockNormal(t *testing.T) {
	tls := NewTLS()
	defer tls.Close()

	m := Xmalloc(tls, 64) // zeroed type field is PTHREAD_MUTEX_NORMAL
	defer Xfree(tls, m)
	Xmemset(tls, m, 0, 64)
	defer Xpthread_mutex_destroy(tls, m) // drop the per-address state before free

	if r := Xpthread_mutex_trylock(tls, m); r != 0 {
		t.Fatalf("trylock on free mutex: %d", r)
	}
	if r := Xpthread_mutex_trylock(tls, m); r != errno.EBUSY {
		t.Fatalf("trylock on held mutex: %d want EBUSY", r)
	}
	if r := Xpthread_mutex_unlock(tls, m); r != 0 {
		t.Fatalf("unlock: %d", r)
	}
	if r := Xpthread_mutex_lock(tls, m); r != 0 {
		t.Fatalf("lock after unlock: %d", r)
	}
	if r := Xpthread_mutex_trylock(tls, m); r != errno.EBUSY {
		t.Fatalf("trylock on locked mutex: %d want EBUSY", r)
	}
	Xpthread_mutex_unlock(tls, m)
}
