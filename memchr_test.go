// Copyright 2026 The Libc Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package libc // import "modernc.org/libc"

import (
	"math/rand"
	"strconv"
	"testing"
	"unsafe"

	"modernc.org/libc/sys/types"
)

// memchrRef is the former byte-at-a-time Xmemchr.
func memchrRef(s uintptr, c int32, n types.Size_t) uintptr {
	for ; n != 0; n-- {
		if *(*byte)(unsafe.Pointer(s)) == byte(c) {
			return s
		}
		s++
	}
	return 0
}

func TestMemchrRandom(t *testing.T) {
	tls := NewTLS()
	defer tls.Close()

	const bufSize = 3 * 4096
	p := Xmalloc(tls, bufSize)
	defer Xfree(tls, p)
	pb := unsafe.Slice((*byte)(unsafe.Pointer(p)), bufSize)

	cs := []int32{0, 1, 'a', 0x7f, 0x80, 0xff, 0x100, 0x161, -1, -256, -0x9f}
	rng := rand.New(rand.NewSource(1))
	for i := 0; i < 20000; i++ {
		// A sparse alphabet gives both early, late and missing matches.
		density := 1 + rng.Intn(5000)
		for j := range pb {
			pb[j] = 'x'
			if rng.Intn(density) == 0 {
				pb[j] = byte(rng.Intn(256))
			}
		}
		off := uintptr(rng.Intn(4096 + 64))
		n := types.Size_t(rng.Intn(bufSize - int(off) + 1))
		c := cs[rng.Intn(len(cs))]
		if got, want := Xmemchr(tls, p+off, c, n), memchrRef(p+off, c, n); got != want {
			t.Fatalf("off=%d n=%d c=%#x: got %#x, want %#x", off, n, c, got, want)
		}
	}
}

func BenchmarkMemchr(b *testing.B) {
	tls := NewTLS()
	defer tls.Close()
	for _, impl := range []struct {
		name string
		f    func(uintptr, int32, types.Size_t) uintptr
	}{
		{"new", func(s uintptr, c int32, n types.Size_t) uintptr { return Xmemchr(tls, s, c, n) }},
		{"old", memchrRef},
	} {
		b.Run(impl.name, func(b *testing.B) {
			for _, size := range []int{8, 64, 1024, 65536} {
				b.Run(strconv.Itoa(size), func(b *testing.B) {
					p := Xmalloc(tls, Tsize_t(size)+1)
					pb := unsafe.Slice((*byte)(unsafe.Pointer(p)), size+1)
					for i := range pb {
						pb[i] = 'x'
					}
					// Odd offset, match at the last byte.
					pb[size] = 'y'
					b.Cleanup(func() { Xfree(tls, p) })
					b.SetBytes(int64(size))
					var sink uintptr
					for i := 0; i < b.N; i++ {
						sink += impl.f(p+1, 'y', types.Size_t(size))
					}
					_ = sink
				})
			}
		})
	}
}
