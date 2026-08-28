// Copyright 2026 The Libc Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build linux && (amd64 || arm64)

package libc // import "modernc.org/libc"

import (
	"bytes"
	"encoding/binary"
	"math"
	"unsafe"
)

// This file replaces hot transpiled musl routines with native Go
// implementations. The transpiled mem* routines move at most four
// bytes per load/store; the Go runtime's memmove, memclr, and
// bytes.Compare use much wider (SIMD) operations. The transpiled
// originals are kept in the generated ccgo_linux_{amd64,arm64}.go
// files under ___musl_* names; generator.go renames them as part of
// generation, so make generate maintains this arrangement. Should
// the two ever fall out of sync, the build fails with duplicate or
// missing definitions rather than silently reverting.
// native_musl_test.go verifies these against the ___musl_* originals
// and benchmarks them; the choices below are those measurements'
// winners on linux/amd64 and linux/arm64.

// void *memcpy(void *dest, const void *src, size_t n);
func Xmemcpy(tls *TLS, dest uintptr, src uintptr, n Tsize_t) (r uintptr) {
	if __ccgo_strace {
		trc("tls=%v dest=%v src=%v n=%v, (%v:)", tls, dest, src, n, origin(2))
		defer func() { trc("-> %v", r) }()
	}
	if n != 0 {
		copy(unsafe.Slice((*byte)(unsafe.Pointer(dest)), n), unsafe.Slice((*byte)(unsafe.Pointer(src)), n))
	}
	return dest
}

// void *memmove(void *dest, const void *src, size_t n);
func Xmemmove(tls *TLS, dest uintptr, src uintptr, n Tsize_t) (r uintptr) {
	if __ccgo_strace {
		trc("tls=%v dest=%v src=%v n=%v, (%v:)", tls, dest, src, n, origin(2))
		defer func() { trc("-> %v", r) }()
	}
	if n != 0 {
		copy(unsafe.Slice((*byte)(unsafe.Pointer(dest)), n), unsafe.Slice((*byte)(unsafe.Pointer(src)), n))
	}
	return dest
}

// void *memset(void *s, int c, size_t n);
func Xmemset(tls *TLS, dest uintptr, c int32, n Tsize_t) (r uintptr) {
	if __ccgo_strace {
		trc("tls=%v dest=%v c=%v n=%v, (%v:)", tls, dest, c, n, origin(2))
		defer func() { trc("-> %v", r) }()
	}
	if n == 0 {
		return dest
	}
	b := unsafe.Slice((*byte)(unsafe.Pointer(dest)), n)
	bc := byte(c)
	if bc == 0 {
		clear(b)
		return dest
	}
	if len(b) < 8 {
		for i := range b {
			b[i] = bc
		}
		return dest
	}
	// Fill the first min(n, 512) bytes with word stores of the
	// replicated fill byte, then repeatedly double the filled prefix
	// with copy, which runs as SIMD runtime.memmove.
	x := 0x0101010101010101 * uint64(bc)
	seed := min(len(b), 512)
	i := 0
	for ; i+32 <= seed; i += 32 {
		q := b[i : i+32 : i+32]
		binary.LittleEndian.PutUint64(q[0:], x)
		binary.LittleEndian.PutUint64(q[8:], x)
		binary.LittleEndian.PutUint64(q[16:], x)
		binary.LittleEndian.PutUint64(q[24:], x)
	}
	for ; i+8 <= seed; i += 8 {
		binary.LittleEndian.PutUint64(b[i:], x)
	}
	binary.LittleEndian.PutUint64(b[seed-8:], x)
	for i := seed; i < len(b); i *= 2 {
		copy(b[i:], b[:i])
	}
	return dest
}

// int memcmp(const void *vl, const void *vr, size_t n);
func Xmemcmp(tls *TLS, vl uintptr, vr uintptr, n Tsize_t) (r1 int32) {
	if __ccgo_strace {
		trc("tls=%v vl=%v vr=%v n=%v, (%v:)", tls, vl, vr, n, origin(2))
		defer func() { trc("-> %v", r1) }()
	}
	if n == 0 {
		return 0
	}
	l := unsafe.Slice((*byte)(unsafe.Pointer(vl)), n)
	r := unsafe.Slice((*byte)(unsafe.Pointer(vr)), n)
	// bytes.Compare returns -1/0/+1 rather than musl's difference of
	// the first mismatched bytes; C99 7.21.4 specifies only the sign.
	return int32(bytes.Compare(l, r))
}

// double fabs(double x);
func Xfabs(tls *TLS, x float64) (r float64) {
	if __ccgo_strace {
		trc("tls=%v x=%v, (%v:)", tls, x, origin(2))
		defer func() { trc("-> %v", r) }()
	}
	r = math.Abs(x)
	if tls.checkSignals {
		tls.checkSignal()
	}
	return r
}

// size_t strcspn(const char *s, const char *c);
func Xstrcspn(tls *TLS, s uintptr, c uintptr) (r Tsize_t) {
	if __ccgo_strace {
		trc("tls=%v s=%v c=%v, (%v:)", tls, s, c, origin(2))
		defer func() { trc("-> %v", r) }()
	}
	var set [4]uint64
	for p := c; ; p++ {
		ch := *(*byte)(unsafe.Pointer(p))
		if ch == 0 {
			break
		}
		set[ch>>6] |= 1 << (ch & 63)
	}
	for q := s; ; q++ {
		ch := *(*byte)(unsafe.Pointer(q))
		if ch == 0 || set[ch>>6]&(1<<(ch&63)) != 0 {
			r = Tsize_t(q - s)
			if tls.checkSignals {
				tls.checkSignal()
			}
			return r
		}
	}
}
