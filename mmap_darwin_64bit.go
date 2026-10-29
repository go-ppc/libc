// Copyright 2026 The Libc Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// The mmap(2) call behind Xmmap for darwin/amd64, darwin/arm64 and
// darwin/ppc64: LP64, so the 64-bit off_t is a single argument and the six
// Syscall6 words are the whole call. darwin/ppc32 differs and lives in
// mmap_darwin_ppc.go.

//go:build darwin && (amd64 || arm64 || ppc64)

package libc // import "modernc.org/libc"

import "golang.org/x/sys/unix"

// xmmap is the mmap(2) call behind Xmmap.
func xmmap(addr uintptr, length uintptr, prot, flags, fd int, offset int64) (uintptr, error) {
	data, _, err := unix.Syscall6(unix.SYS_MMAP, addr, length, uintptr(prot), uintptr(flags), uintptr(fd), uintptr(offset))
	if err != 0 {
		return 0, err
	}
	return data, nil
}
