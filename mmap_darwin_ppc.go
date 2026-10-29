// Copyright 2026 The Libc Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// The mmap(2) call behind Xmmap for darwin/ppc32, where Leopard's off_t is 64
// bits wide and therefore does not fit in one argument word. The other three
// Darwin targets share mmap_darwin_64bit.go.

//go:build darwin && ppc

package libc // import "modernc.org/libc"

import (
	"unsafe"

	"golang.org/x/sys/unix"
)

// xmmap is the mmap(2) call behind Xmmap, split per platform because the
// 64-bit off_t argument does not reach the kernel the same way everywhere.
//
// On ppc32 Leopard, off_t is __int64_t (sys/_types.h:110, sys/mman.h:203), so
// mmap(void *, size_t, int, int, int, off_t) takes seven argument words. The
// kernel reads them from R3..R9 -- bsd/dev/ppc/syscalls.c:158 calls the
// argument munger of bsd/kern/syscalls.master:197 over the saved registers, and
// _munge_wwwwwl (bsd/dev/ppc/munge.s) loads R3, R4, R5, R6, R7 and then the
// 64-bit pos from R8 (high) and R9 (low). A one-word pass therefore puts the
// off_t in the high word and leaves the low word to whatever the caller had in
// R9; kern_mman.c:250,576 then maps from that garbage offset and returns
// page-base + pageoff, an address munmap(2) refuses as unaligned. Measured on
// the G4 with a C probe driving the same system call (runs/
// libc-mmap-fix-20261029/receipt.txt).
//
// unix.MmapPtr is the address-preserving form of the x/sys mmap wrapper, whose
// darwin/ppc32 body already passes the off_t as those two words
// (unix/zsyscall_darwin_ppc.go:2062-2064), so this file carries no ABI
// arithmetic of its own.
func xmmap(addr uintptr, length uintptr, prot, flags, fd int, offset int64) (uintptr, error) {
	p, err := unix.MmapPtr(fd, offset, unsafe.Pointer(addr), length, prot, flags)
	return uintptr(p), err
}
