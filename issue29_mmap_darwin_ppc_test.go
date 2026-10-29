// Copyright 2026 The Libc Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build darwin && ppc

package libc

import (
	"os"
	"path/filepath"
	"testing"
	"unsafe"

	"golang.org/x/sys/unix"
	"modernc.org/libc/sys/types"
)

// TestIssue29MmapOffset is the ppc32 half of TestIssue29. Leopard's off_t is 64
// bits wide at ppc32 (sys/_types.h:110), so mmap(2) needs two argument words
// for it, and the kernel reads them from R8 (high) and R9 (low). Passing one
// word puts the offset in the high word and lets the low word keep whatever the
// caller had there; the kernel then maps from that offset and returns
// page-base + pageoff, which munmap(2) rejects as unaligned.
//
// Offset 0 alone cannot show the high word: only the low word decides pageoff.
// A non-zero, page-aligned offset, with the bytes at that offset compared
// against the file, is what makes both words observable.
func TestIssue29MmapOffset(t *testing.T) {
	const page = 4096

	dir := t.TempDir()
	fn := filepath.Join(dir, "test")
	want := make([]byte, 3*page)
	for p := 0; p < 3; p++ {
		for i := 0; i < page; i++ {
			want[p*page+i] = pageByte(p)
		}
	}
	if err := os.WriteFile(fn, want, 0644); err != nil {
		t.Fatal(err)
	}
	f, err := os.OpenFile(fn, os.O_RDWR, 0644)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	tls := NewTLS()
	defer tls.Close()

	for _, off := range []int64{0, page} {
		first := pageByte(int(off / page))
		d := Xmmap(tls, 0, page, unix.PROT_READ, unix.MAP_SHARED, int32(f.Fd()), types.Off_t(off))
		if d == ^uintptr(0) {
			t.Errorf("offset %d: mmap failed", off)
			continue
		}
		if pageoff := d & (page - 1); pageoff != 0 {
			t.Errorf("offset %d: mmap returned %#x, which is %#x bytes into a page; munmap cannot unmap it", off, d, pageoff)
			continue
		}
		if got := *(*byte)(unsafe.Pointer(d)); got != first {
			t.Errorf("offset %d: first byte is %#x, want %#x", off, got, first)
		}
		if rc := Xmunmap(tls, d, page); rc != 0 {
			t.Errorf("offset %d: munmap(%#x) failed: %d", off, d, rc)
		}
	}
}

// pageByte is the value TestIssue29MmapOffset fills page p with.
func pageByte(p int) byte { return byte(0xa0 + p) }
