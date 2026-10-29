// Copyright 2020 The Libc Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// getloadavg for the LP64 Darwin targets, where vm.loadavg is 24 bytes:
// fixpt_t ldavg[3] (12 bytes), four bytes of padding, then an eight-byte long
// fscale at offset 16. darwin/ppc32 differs and lives in
// getloadavg_darwin_ppc.go.

//go:build darwin && (amd64 || arm64 || ppc64)

package libc // import "modernc.org/libc"

import (
	"unsafe"

	"golang.org/x/sys/unix"
)

// int getloadavg(double loadavg[], int nelem);
func Xgetloadavg(t *TLS, loadavg uintptr, nelem int32) int32 {
	if __ccgo_strace {
		trc("t=%v loadavg=%v nelem=%v, (%v:)", t, loadavg, nelem, origin(2))
	}
	// struct loadavg { fixpt_t ldavg[3]; long fscale; }
	b, err := unix.SysctlRaw("vm.loadavg")
	if err != nil || len(b) < 24 {
		return -1
	}

	fscale := float64(*(*int64)(unsafe.Pointer(&b[16])))
	n := int32(0)
	for ; n < nelem && n < 3; n++ {
		*(*float64)(unsafe.Pointer(loadavg + uintptr(n)*8)) = float64(*(*uint32)(unsafe.Pointer(&b[4*n]))) / fscale
	}
	return n
}
