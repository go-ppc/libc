// Copyright 2026 The Libc Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build (linux && (amd64 || arm64 || loong64 || ppc64le || s390x || riscv64 || 386 || arm)) || freebsd || netbsd || openbsd

package libc // import "modernc.org/libc"

import (
	"os"
	"path/filepath"
	"testing"
	"unsafe"

	"modernc.org/libc/errno"
)

// fdatasync syncs a regular file and a directory, which is what SQLite's unix
// VFS asks of it, and fails with EBADF on a descriptor that is not open.
func TestFdatasync(t *testing.T) {
	tls := NewTLS()
	defer tls.Close()

	errnoOf := func() int32 { return *(*int32)(unsafe.Pointer(X__errno_location(tls))) }
	dir := t.TempDir()
	f, err := os.Create(filepath.Join(dir, "f"))
	if err != nil {
		t.Fatal(err)
	}

	defer f.Close()
	if _, err := f.Write(make([]byte, 1<<12)); err != nil {
		t.Fatal(err)
	}

	if rc := Xfdatasync(tls, int32(f.Fd())); rc != 0 {
		t.Errorf("fdatasync of a file: %d, errno %d", rc, errnoOf())
	}

	d, err := os.Open(dir)
	if err != nil {
		t.Fatal(err)
	}

	defer d.Close()
	if rc := Xfdatasync(tls, int32(d.Fd())); rc != 0 {
		t.Errorf("fdatasync of a directory: %d, errno %d", rc, errnoOf())
	}

	if rc, e := Xfdatasync(tls, -1), errnoOf(); rc != -1 || e != errno.EBADF {
		t.Errorf("fdatasync(-1): %d, errno %d, want -1, errno %d", rc, e, errno.EBADF)
	}
}
