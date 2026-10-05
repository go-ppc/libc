// Copyright 2026 The Libc Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package libc // import "modernc.org/libc"

import (
	"bytes"
	"math/rand"
	"testing"
	"unsafe"
)

// guarded returns a buffer of n bytes surrounded by guard bytes
func guarded(tls *TLS, n int) (p uintptr, b []byte, free func()) {
	const guard = 16
	base := Xmalloc(tls, Tsize_t(n+2*guard))
	all := unsafe.Slice((*byte)(unsafe.Pointer(base)), n+2*guard)
	for i := range all {
		all[i] = 0xee
	}
	return base + guard, all[guard : guard+n], func() { Xfree(tls, base) }
}

func TestMemcmpFastPath(t *testing.T) {
	tls := NewTLS()
	defer tls.Close()

	rng := rand.New(rand.NewSource(1))
	for _, n := range []int{0, 1, 2, 7, 8, 15, 16, 63, 64, 1000, 4097} {
		p1, b1, f1 := guarded(tls, n)
		p2, b2, f2 := guarded(tls, n)
		rng.Read(b1)
		copy(b2, b1)

		if got := Xmemcmp(tls, p1, p2, Tsize_t(n)); got != 0 {
			t.Fatalf("n=%d equal: got %d", n, got)
		}

		for k := 0; k < n && k < 8; k++ {
			i := rng.Intn(n)
			b2[i] ^= 1 + byte(rng.Intn(255))
			want := bytes.Compare(b1, b2)
			got := Xmemcmp(tls, p1, p2, Tsize_t(n))
			if (got < 0) != (want < 0) || (got > 0) != (want > 0) {
				t.Fatalf("n=%d i=%d: got %d want sign %d", n, i, got, want)
			}
			b2[i] = b1[i]
		}
		f1()
		f2()
	}
}

func TestStrrchrFastPath(t *testing.T) {
	tls := NewTLS()
	defer tls.Close()

	for _, tc := range []struct {
		s    string
		c    int32
		want int // -1: not found
	}{
		{"", 'a', -1},
		{"", 0, 0},
		{"a", 'a', 0},
		{"abc", 'x', -1},
		{"abcabc", 'a', 3},
		{"abcabc", 'c', 5},
		{"/usr/lib/libc.so", '/', 8},
		{"abc", 0, 3}, // the terminator is part of the string
		{"abc", 0x161, 0},
		{"aaaa", 'a', 3},
	} {
		p, err := CString(tc.s)
		if err != nil {
			t.Fatal(err)
		}

		want := uintptr(0)
		if tc.want >= 0 {
			want = p + uintptr(tc.want)
		}
		if got := Xstrrchr(tls, p, tc.c); got != want {
			t.Errorf("strrchr(%q, %#x): got %d want %d", tc.s, tc.c, int64(got)-int64(p), tc.want)
		}
		Xfree(tls, p)
	}
}
