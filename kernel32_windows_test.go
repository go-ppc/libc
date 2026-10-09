// Copyright 2026 The Libc Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build windows

package libc // import "modernc.org/libc"

import (
	"os"
	"path/filepath"
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"
)

// The kernel32 functions SQLite 3.54.0 references unconditionally.

func kernel32TestFile(t *testing.T, size int) windows.Handle {
	fn := filepath.Join(t.TempDir(), "f")
	if err := os.WriteFile(fn, make([]byte, size), 0o600); err != nil {
		t.Fatal(err)
	}

	h, err := windows.CreateFile(windows.StringToUTF16Ptr(fn), windows.GENERIC_READ|windows.GENERIC_WRITE, 0, nil, windows.OPEN_EXISTING, windows.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() { windows.CloseHandle(h) })
	return h
}

func TestGetFileSizeEx(t *testing.T) {
	tls := NewTLS()
	defer tls.Close()

	const size = 12345
	h := kernel32TestFile(t, size)
	p := tls.Alloc(8)
	defer tls.Free(8)

	if rc := XGetFileSizeEx(tls, uintptr(h), p); rc == 0 {
		t.Fatalf("GetFileSizeEx failed: %v", XGetLastError(tls))
	}

	if g := *(*int64)(unsafe.Pointer(p)); g != size {
		t.Fatalf("size %v, want %v", g, size)
	}

	if rc := XGetFileSizeEx(tls, uintptr(windows.InvalidHandle), p); rc != 0 {
		t.Fatal("GetFileSizeEx of an invalid handle succeeded")
	}

	if g := XGetLastError(tls); g != uint32(windows.ERROR_INVALID_HANDLE) {
		t.Fatalf("GetLastError %v, want %v", g, uint32(windows.ERROR_INVALID_HANDLE))
	}
}

func TestSetFilePointerEx(t *testing.T) {
	tls := NewTLS()
	defer tls.Close()

	const size = 100
	h := kernel32TestFile(t, size)
	p := tls.Alloc(8)
	defer tls.Free(8)

	li := func(d int64) (r largeInteger) {
		r.F__ccgo0_0.FLowPart = uint32(d)
		r.F__ccgo0_0.FHighPart = int32(d >> 32)
		return r
	}
	pos := func() int64 {
		if rc := XSetFilePointerEx(tls, uintptr(h), li(0), p, windows.FILE_CURRENT); rc == 0 {
			t.Fatalf("SetFilePointerEx(0, FILE_CURRENT) failed: %v", XGetLastError(tls))
		}

		return *(*int64)(unsafe.Pointer(p))
	}

	// Distances at and past 4 GiB need both halves of the LARGE_INTEGER, and
	// 0xffffffff in the low half catches a sign-extended LowPart. Moving the
	// pointer past the end of the file does not extend it.
	for _, d := range []int64{3, 4 << 30, 5<<30 + 7, 1<<32 | 0xffffffff, 1 << 40} {
		// As SQLite calls it: a NULL lpNewFilePointer and FILE_BEGIN.
		if rc := XSetFilePointerEx(tls, uintptr(h), li(d), 0, windows.FILE_BEGIN); rc == 0 {
			t.Fatalf("SetFilePointerEx(%#x, FILE_BEGIN) failed: %v", d, XGetLastError(tls))
		}

		if g := pos(); g != d {
			t.Fatalf("position %#x, want %#x", g, d)
		}

		if rc := XSetFilePointerEx(tls, uintptr(h), li(-3), p, windows.FILE_CURRENT); rc == 0 {
			t.Fatalf("SetFilePointerEx(-3, FILE_CURRENT) failed: %v", XGetLastError(tls))
		}

		if g := *(*int64)(unsafe.Pointer(p)); g != d-3 {
			t.Fatalf("new position %#x, want %#x", g, d-3)
		}
	}

	if rc := XSetFilePointerEx(tls, uintptr(h), li(5<<30), p, windows.FILE_END); rc == 0 {
		t.Fatalf("SetFilePointerEx(5 GiB, FILE_END) failed: %v", XGetLastError(tls))
	}

	if g, e := *(*int64)(unsafe.Pointer(p)), int64(size+5<<30); g != e {
		t.Fatalf("new position %#x, want %#x", g, e)
	}

	if rc := XGetFileSizeEx(tls, uintptr(h), p); rc == 0 || *(*int64)(unsafe.Pointer(p)) != size {
		t.Fatalf("seeking changed the file size: %v", *(*int64)(unsafe.Pointer(p)))
	}

	// A negative absolute position fails and sets the error.
	if rc := XSetFilePointerEx(tls, uintptr(h), li(-1), 0, windows.FILE_BEGIN); rc != 0 {
		t.Fatal("SetFilePointerEx(-1, FILE_BEGIN) succeeded")
	}

	if g := XGetLastError(tls); g != uint32(windows.ERROR_NEGATIVE_SEEK) {
		t.Fatalf("GetLastError %v, want %v", g, uint32(windows.ERROR_NEGATIVE_SEEK))
	}
}

func TestGetTickCount64(t *testing.T) {
	tls := NewTLS()
	defer tls.Close()

	a := XGetTickCount(tls)
	b := XGetTickCount64(tls)
	c := XGetTickCount(tls)
	// The low half is GetTickCount; on 386 the high half comes from EDX, so
	// also check it holds no garbage: ten years of uptime is under 2^39 ms.
	if lo := uint32(b); lo-a > c-a {
		t.Fatalf("GetTickCount64 low half %v outside GetTickCount [%v, %v]", lo, a, c)
	}

	if b >= 1<<39 {
		t.Fatalf("GetTickCount64 %#x", b)
	}
}

func TestGetDriveTypeW(t *testing.T) {
	tls := NewTLS()
	defer tls.Close()

	root := filepath.VolumeName(os.TempDir()) + `\`
	for _, s := range []string{root, `C:\no such directory\`} {
		if g, e := XGetDriveTypeW(tls, allocW(tls, s)), windows.GetDriveType(windows.StringToUTF16Ptr(s)); g != e {
			t.Fatalf("GetDriveTypeW(%q) = %v, want %v", s, g, e)
		}
	}

	if g := XGetDriveTypeW(tls, allocW(tls, root)); g == windows.DRIVE_UNKNOWN || g == windows.DRIVE_NO_ROOT_DIR {
		t.Fatalf("GetDriveTypeW(%q) = %v", root, g)
	}

	if g, e := XGetDriveTypeW(tls, 0), windows.GetDriveType(nil); g != e {
		t.Fatalf("GetDriveTypeW(NULL) = %v, want %v", g, e)
	}
}
