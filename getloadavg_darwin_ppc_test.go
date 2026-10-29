// Copyright 2026 The Libc Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build darwin && ppc

package libc

import (
	"math"
	"testing"
	"unsafe"

	"golang.org/x/sys/unix"
)

// TestGetloadavgPPC32Shape pins the ILP32 shape of the vm.loadavg payload and
// shows that Xgetloadavg fills the caller's array from it. Leopard declares
//
//	struct loadavg { fixpt_t ldavg[3]; long fscale; };  // sys/sysctl.h:528-531
//
// with `typedef u_int32_t fixpt_t` (sys/types.h:107), so at ppc32 the struct is
// 12 + 4 = 16 bytes and not the 24-byte LP64 fixpt_t[3]/padding/long. The G4 run
// in runs/libc-g4-20261029/receipt.txt is what this pins: the LP64 reading
// returned -1 and left the buffer uninitialised.
func TestGetloadavgPPC32Shape(t *testing.T) {
	b, err := unix.SysctlRaw("vm.loadavg")
	if err != nil {
		t.Fatalf("sysctl vm.loadavg: %v", err)
	}
	if len(b) != 16 {
		t.Fatalf("vm.loadavg returned %d bytes, want 16 (fixpt_t ldavg[3] + 4-byte long)", len(b))
	}
	fscale := float64(*(*int32)(unsafe.Pointer(&b[12])))
	if fscale <= 0 {
		t.Fatalf("vm.loadavg fscale is %v, want a positive fixed-point scale", fscale)
	}
	t.Logf("vm.loadavg: % x fscale=%v", b, fscale)

	const nelem = 3
	tls := NewTLS()
	defer tls.Close()

	p := Xmalloc(tls, nelem*8)
	defer Xfree(tls, p)
	// 0x7f bytes are a NaN: a value that was never written stays a NaN.
	Xmemset(tls, p, 0x7f, nelem*8)

	if got := Xgetloadavg(tls, p, nelem); got != nelem {
		t.Fatalf("getloadavg returned %v, want %v", got, nelem)
	}
	avg := make([]float64, nelem)
	for i := range avg {
		avg[i] = *(*float64)(unsafe.Pointer(p + uintptr(i)*8))
		if math.IsNaN(avg[i]) || math.IsInf(avg[i], 0) || avg[i] < 0 {
			t.Errorf("loadavg[%d] = %v, want a non-negative finite load average", i, avg[i])
		}
	}
	t.Logf("getloadavg (1/5/15 min): %v", avg)
}
