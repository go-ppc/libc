// Copyright 2026 The Libc Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unix && !(linux && (amd64 || arm64 || loong64 || ppc64le || s390x || riscv64 || 386 || arm))

package libc // import "modernc.org/libc"

import (
	"os"
	"testing"
	"unsafe"

	"golang.org/x/sys/unix"
)

// TestStrcopyGuardPage places the source right before a PROT_NONE page,
// so any read past the NUL (or past n for strncpy/strncat) faults.
func TestStrcopyGuardPage(t *testing.T) {
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
	d := Xmalloc(tls, Tsize_t(pg+16))
	defer Xfree(tls, d)
	dst := unsafe.Slice((*byte)(unsafe.Pointer(d)), pg+16)
	for i := 0; i < pg; i++ {
		mem[i] = 'x'
	}
	for _, n := range []int{0, 1, 2, 7, 8, 9, 63, 64, 65, 511, 4095} {
		if n >= pg {
			continue
		}
		// Unterminated source ending at the guard page, bounded by n
		// (n == 0 would point at the guard page itself).
		mem[pg-1] = 'x'
		if n > 0 {
			s := end - uintptr(n)
			Xstrncpy(tls, d, s, Tsize_t(n))
			dst[0] = 0
			strncat(d, s, Tsize_t(n))
			if got := strlen(d); got != Tsize_t(n) {
				t.Fatalf("strncat unterminated: n=%d got=%d", n, got)
			}
		}

		// NUL is the last byte before the guard page.
		mem[pg-1] = 0
		s := end - uintptr(n) - 1
		if got := Xstrchr(tls, s, 'y'); got != 0 {
			t.Fatalf("strchr: n=%d got=%#x", n, got)
		}
		if got := Xstrchr(tls, s, 0); got != end-1 {
			t.Fatalf("strchr NUL: n=%d got=%#x want %#x", n, got, end-1)
		}
		Xstrncpy(tls, d, s, Tsize_t(n+10))
		dst[0] = 0
		strncat(d, s, Tsize_t(n+10))
		Xstrcpy(tls, d, s)
		if got := Xstrstr(tls, s, s); got != s {
			t.Fatalf("strstr: n=%d got=%#x want %#x", n, got, s)
		}
	}
}
