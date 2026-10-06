// Copyright 2026 The Libc Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unix

package libc // import "modernc.org/libc"

import (
	"os"
	"testing"
	"unsafe"

	"golang.org/x/sys/unix"
	"modernc.org/libc/sys/types"
)

// TestMemchrGuardPage verifies that memchr and strnlen stop at the
// first match when n exceeds the object, up to SIZE_MAX: the match is
// the last byte before a PROT_NONE guard page, where any overread faults.
func TestMemchrGuardPage(t *testing.T) {
	tls := NewTLS()
	defer tls.Close()

	pg := os.Getpagesize()
	mem, err := unix.Mmap(-1, 0, 2*pg, unix.PROT_READ|unix.PROT_WRITE, unix.MAP_ANON|unix.MAP_PRIVATE)
	if err != nil {
		t.Fatal(err)
	}
	defer unix.Munmap(mem)
	if err := unix.Mprotect(mem[pg:], unix.PROT_NONE); err != nil {
		t.Fatal(err)
	}

	base := uintptr(unsafe.Pointer(&mem[0]))
	end := base + uintptr(pg)
	for i := 0; i < pg-1; i++ {
		mem[i] = 'x'
	}
	mem[pg-1] = 0
	for _, n := range []int{0, 1, 2, 7, 8, 9, 63, 64, 65, 511, 4095} {
		if n >= pg {
			continue
		}
		s := end - uintptr(n) - 1
		if got := Xmemchr(tls, s, 0, ^types.Size_t(0)); got != end-1 {
			t.Fatalf("memchr: n=%d got=%#x want=%#x", n, got, end-1)
		}
		if got := Xmemchr(tls, s, 0x100, ^types.Size_t(0)); got != end-1 {
			t.Fatalf("memchr(0x100): n=%d got=%#x want=%#x", n, got, end-1)
		}
		if got := Xstrnlen(tls, s, ^Tsize_t(0)); got != Tsize_t(n) {
			t.Fatalf("strnlen: n=%d got=%d", n, got)
		}
		// No match within the object: n ends exactly at the guard page.
		if got := Xmemchr(tls, s, 'y', types.Size_t(n+1)); got != 0 {
			t.Fatalf("memchr miss: n=%d got=%#x", n, got)
		}
	}
}
