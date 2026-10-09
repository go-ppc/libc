// Copyright 2026 The Libc Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package libc // import "modernc.org/libc"

import (
	"bytes"
	"fmt"
	"math/rand"
	"slices"
	"testing"
	"unsafe"
)

// qsortFuncPtr returns f as a C function pointer for Xqsort.
func qsortFuncPtr(f func(*TLS, uintptr, uintptr) int32) uintptr {
	return *(*uintptr)(unsafe.Pointer(&struct {
		f func(*TLS, uintptr, uintptr) int32
	}{f}))
}

// qsortCmpInt32 orders elements by their leading int32.
func qsortCmpInt32(t *TLS, a, b uintptr) int32 {
	x, y := *(*int32)(unsafe.Pointer(a)), *(*int32)(unsafe.Pointer(b))
	switch {
	case x < y:
		return -1
	case x > y:
		return 1
	}
	return 0
}

// qsortCmpBytes orders elements as byte strings; the width is in the
// closure so that every byte of the element takes part.
func qsortCmpBytes(width int) func(*TLS, uintptr, uintptr) int32 {
	return func(t *TLS, a, b uintptr) int32 {
		return int32(bytes.Compare(unsafe.Slice((*byte)(unsafe.Pointer(a)), width), unsafe.Slice((*byte)(unsafe.Pointer(b)), width)))
	}
}

func TestQsort(t *testing.T) {
	tls := NewTLS()
	defer tls.Close()

	rng := rand.New(rand.NewSource(1))
	// Widths that select the 8-byte, the 4-byte and the bytewise swap,
	// at base offsets that do or do not keep their alignment.
	for _, width := range []int{1, 3, 4, 8, 12, 16, 24, 64, 100} {
		for _, off := range []int{0, 4, 8, 1} {
			for _, nel := range []int{0, 1, 2, 3, 7, 100, 1000} {
				buf := Xmalloc(tls, Tsize_t(off+nel*width+1))
				base := buf + uintptr(off)
				b := unsafe.Slice((*byte)(unsafe.Pointer(base)), nel*width)
				rng.Read(b)
				b = append(b[:0:0], b...) // the unsorted copy
				cmp := qsortCmpBytes(width)
				Xqsort(tls, base, Tsize_t(nel), Tsize_t(width), qsortFuncPtr(cmp))
				got := unsafe.Slice((*byte)(unsafe.Pointer(base)), nel*width)
				want := make([][]byte, nel)
				for i := range want {
					want[i] = b[i*width : (i+1)*width]
				}
				slices.SortFunc(want, bytes.Compare)
				if !bytes.Equal(got, bytes.Join(want, nil)) {
					t.Fatalf("width=%d off=%d nel=%d: got %x, want %x", width, off, nel, got, bytes.Join(want, nil))
				}
				Xfree(tls, buf)
			}
		}
	}
}

// TestQsortElementPointers checks that compar sees pointers into the
// array: C code may index a parallel array by (p - base) / width.
func TestQsortElementPointers(t *testing.T) {
	tls := NewTLS()
	defer tls.Close()

	const nel, width = 100, 12
	base := Xmalloc(tls, nel*width)
	defer Xfree(tls, base)
	end := base + nel*width
	cmp := func(t *TLS, a, b uintptr) int32 {
		if a < base || a >= end || b < base || b >= end || (a-base)%width != 0 || (b-base)%width != 0 {
			panic(fmt.Sprintf("compar called with %#x, %#x outside [%#x, %#x)", a, b, base, end))
		}
		return qsortCmpInt32(t, a, b)
	}
	for i := 0; i < nel; i++ {
		*(*int32)(unsafe.Pointer(base + uintptr(i*width))) = int32(nel - i)
	}
	Xqsort(tls, base, nel, width, qsortFuncPtr(cmp))
	for i := 0; i < nel; i++ {
		if got := *(*int32)(unsafe.Pointer(base + uintptr(i*width))); got != int32(i+1) {
			t.Fatalf("element %d = %d", i, got)
		}
	}
}

func BenchmarkQsort(b *testing.B) {
	tls := NewTLS()
	defer tls.Close()
	for _, width := range []int{4, 16, 64} {
		for _, nel := range []int{100, 10000} {
			base := Xmalloc(tls, Tsize_t(nel*width))
			unsorted := make([]byte, nel*width)
			rng := rand.New(rand.NewSource(1))
			for i := 0; i < nel; i++ {
				*(*int32)(unsafe.Pointer(&unsorted[i*width])) = rng.Int31()
			}
			cmp := qsortFuncPtr(qsortCmpInt32)
			b.Run(fmt.Sprintf("width%d/nel%d", width, nel), func(b *testing.B) {
				for i := 0; i < b.N; i++ {
					copy(unsafe.Slice((*byte)(unsafe.Pointer(base)), nel*width), unsorted)
					Xqsort(tls, base, Tsize_t(nel), Tsize_t(width), cmp)
				}
			})
			Xfree(tls, base)
		}
	}
}
