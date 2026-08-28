// Copyright 2026 The Libc Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package libc // import "modernc.org/libc"

import (
	"math/rand"
	"strconv"
	"testing"
	"unsafe"
)

// strlenWords is the previous strlen implementation, kept as a
// benchmark baseline: byte-wise to 8-byte alignment, then aligned
// word loads (which cannot cross a page boundary and so cannot
// fault) with the usual zero-in-word bit test.
func strlenWords(s uintptr) Tsize_t {
	if s == 0 {
		return 0
	}
	p := s
	for ; p%8 != 0; p++ {
		if *(*byte)(unsafe.Pointer(p)) == 0 {
			return Tsize_t(p - s)
		}
	}
	for {
		x := *(*uint64)(unsafe.Pointer(p))
		if (x-0x0101010101010101)&^x&0x8080808080808080 != 0 {
			break
		}
		p += 8
	}
	for *(*byte)(unsafe.Pointer(p)) != 0 {
		p++
	}
	return Tsize_t(p - s)
}

func TestStrlenRandom(t *testing.T) {
	tls := NewTLS()
	defer tls.Close()

	const bufSize = 8192
	p := Xmalloc(tls, bufSize)
	defer Xfree(tls, p)
	pb := unsafe.Slice((*byte)(unsafe.Pointer(p)), bufSize)

	rng := rand.New(rand.NewSource(1))
	for i := 0; i < 10000; i++ {
		for j := range pb {
			pb[j] = byte(1 + rng.Intn(255))
		}
		n := rng.Intn(bufSize - 64)
		off := uintptr(rng.Intn(32))
		pb[off+uintptr(n)] = 0
		if got := strlen(p + off); got != Tsize_t(n) {
			t.Fatalf("strlen: n=%d off=%d got=%d", n, off, got)
		}
		if got := strlenWords(p + off); got != Tsize_t(n) {
			t.Fatalf("strlenWords: n=%d off=%d got=%d", n, off, got)
		}
	}
}

func BenchmarkStrlen(b *testing.B) {
	tls := NewTLS()
	defer tls.Close()
	for _, impl := range []struct {
		name string
		f    func(uintptr) Tsize_t
	}{
		{"native", strlen},
		{"words", strlenWords},
	} {
		b.Run(impl.name, func(b *testing.B) {
			for _, size := range []int{5, 16, 64, 512, 4096, 65536} {
				b.Run(strconv.Itoa(size), func(b *testing.B) {
					p := Xmalloc(tls, Tsize_t(size)+64)
					pb := unsafe.Slice((*byte)(unsafe.Pointer(p)), size+64)
					for i := range pb {
						pb[i] = 'x'
					}
					// Odd offset: C strings are rarely word-aligned.
					pb[1+size] = 0
					b.Cleanup(func() { Xfree(tls, p) })
					b.SetBytes(int64(size))
					var sink Tsize_t
					for i := 0; i < b.N; i++ {
						sink += impl.f(p + 1)
					}
					_ = sink
				})
			}
		})
	}
}
