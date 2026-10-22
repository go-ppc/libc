// Copyright 2026 The Libc Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !libc.membrk && linux && (amd64 || arm64 || loong64 || ppc64le || s390x || riscv64 || 386 || arm)

package libc // import "modernc.org/libc"

// trimAllocator returns the mappings the allocator retains for reuse to the OS.
func trimAllocator() error {
	allocatorMu.Lock()

	defer allocatorMu.Unlock()

	return allocator.Trim()
}

// tlsStackBlock returns the allocator block holding the frame tls.Alloc
// returned last: on this path the slot it took.
func tlsStackBlock(tls *TLS) uintptr {
	return tls.stack[tls.sp-1].p
}

// tlsAllocaEnter does what the generated code does on entry to a function
// that calls alloca. The matching AllocaExit is skipped on purpose.
func tlsAllocaEnter(tls *TLS) {
	tls.AllocaEntry()
}
