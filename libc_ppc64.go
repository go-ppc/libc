// Copyright 2026 The Libc Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file ports libc_arm.go/libc_386.go to the PowerPC ABI. PowerPC
// guarantees single-copy atomicity for naturally aligned byte and int16
// accesses, so the plain load/store forms those files already justify are
// what is used here: no lwarx/stdcx. reservation loop is needed for 8- and
// 16-bit atomics.

//go:build !linux

package libc // import "modernc.org/libc"

import (
	"fmt"
	"unsafe"
)

// Byte loads are atomic on this CPU.
func a_load_8(addr uintptr) uint32 {
	return uint32(*(*byte)(unsafe.Pointer(addr)))
}

// int16 loads are atomic on this CPU when properly aligned.
func a_load_16(addr uintptr) uint32 {
	if addr&1 != 0 {
		panic(fmt.Errorf("unaligned atomic 16 bit access at %#0x", addr))
	}

	return uint32(*(*uint16)(unsafe.Pointer(addr)))
}

// Byte stores are atomic on this CPU.
func a_store_8(addr uintptr, b byte) {
	*(*byte)(unsafe.Pointer(addr)) = b
}

// int16 stores are atomic on this CPU when properly aligned.
func a_store_16(addr uintptr, n uint16) {
	if addr&1 != 0 {
		panic(fmt.Errorf("unaligned atomic 16 bit access at %#0x", addr))
	}

	*(*uint16)(unsafe.Pointer(addr)) = n
}
