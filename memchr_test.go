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
	for j := range pb {
		pb[j] = 'x'
	}

	cs := []int32{0, 1, 'a', 0x7f, 0x80, 0xff, 0x100, 0x161, -1, -256, -0x9f}
	rng := rand.New(rand.NewSource(1))
	var pos []int
	for i := 0; i < 20000; i++ {
		// A few random bytes in a sea of 'x' give early, late and missing matches.
		c := cs[rng.Intn(len(cs))]
		pos = pos[:0]
		for k := rng.Intn(64); k > 0; k-- {
			j := rng.Intn(bufSize)
			if rng.Intn(4) == 0 {
				pb[j] = byte(c)
			} else {
				pb[j] = byte(rng.Intn(256))
			}
			pos = append(pos, j)
		}
		off := uintptr(rng.Intn(4096 + 64))
		n := types.Size_t(rng.Intn(bufSize - int(off) + 1))
		if got, want := Xmemchr(tls, p+off, c, n), memchrRef(p+off, c, n); got != want {
			t.Fatalf("off=%d n=%d c=%#x: got %#x, want %#x", off, n, c, got, want)
		}
		for _, j := range pos {
			pb[j] = 'x'
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
