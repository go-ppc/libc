// Copyright 2026 The Libc Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// getloadavg for darwin/ppc32, where Leopard's vm.loadavg is 16 bytes and not
// the 24-byte LP64 layout the other three Darwin targets use
// (getloadavg_darwin_64bit.go).

//go:build darwin && ppc

package libc // import "modernc.org/libc"

import (
	"unsafe"

	"golang.org/x/sys/unix"
)

// int getloadavg(double loadavg[], int nelem);
//
// The sysctl payload is Leopard's own declaration,
//
//	struct loadavg { fixpt_t ldavg[3]; long fscale; };  // sys/sysctl.h:528-531
//
// with `typedef u_int32_t fixpt_t` (sys/types.h:107) and a four-byte long
// (ppc/_types.h:118 makes __darwin_time_t a long, which is why time_t is 32
// bits here too), so the struct is 12 + 4 = 16 bytes with fscale last, at
// offset 12. The 24-byte reading — accept >= 24 bytes, fscale an int64 at
// &b[16] — is the LP64 shape: fixpt_t[3], four bytes of padding to an
// eight-byte boundary, eight-byte long. Executed on a G4 it returned -1 on
// every call and left the caller's buffer holding uninitialised memory
// (runs/libc-g4-20261029/receipt.txt, F3); the same reading is correct on
// darwin/amd64, which is where the shared file was written.
//
// The length is required to be exactly 16 rather than merely >= 16: a payload
// of any other size means the struct shape assumed here is not the one the
// kernel is sending, and returning the documented failure value says so
// instead of dividing by whatever lands at offset 12.
func Xgetloadavg(t *TLS, loadavg uintptr, nelem int32) int32 {
	if __ccgo_strace {
		trc("t=%v loadavg=%v nelem=%v, (%v:)", t, loadavg, nelem, origin(2))
	}
	b, err := unix.SysctlRaw("vm.loadavg")
	if err != nil || len(b) != 16 {
		return -1
	}

	fscale := float64(*(*int32)(unsafe.Pointer(&b[12])))
	n := int32(0)
	for ; n < nelem && n < 3; n++ {
		*(*float64)(unsafe.Pointer(loadavg + uintptr(n)*8)) = float64(*(*uint32)(unsafe.Pointer(&b[4*n]))) / fscale
	}
	return n
}
