// Copyright 2026 The Libc Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// musl's <stdio.h> leaves FILE opaque, so ccgo emits no FILE for linux/ppc.
// libc_linux.go allocates and reads its streams through stdio.FILE, and the
// translated musl stdio code in musl_linux_ppc.go (__lockfile, __toread,
// __uflow) works on the same objects through musl's internal struct _IO_FILE
// (musl/src/internal/stdio_impl.h). This is that struct, with the two fields
// libc_linux.go names kept under the glibc spellings it was written against:
// F_flags (musl's flags, int32 because libc_linux.go's flags helpers are) and
// F_fileno (musl's fd). TestLinuxPPCFILELayout checks it against the
// translated _IO_FILE.

package stdio

const (
	// musl/src/internal/stdio_impl.h:17: #define F_ERR 32, a bit of flags,
	// the same value and field as glibc's _IO_ERR_SEEN.
	X_IO_ERR_SEEN = 32
)

type FILE = struct {
	F_flags       int32
	Frpos         uintptr
	Frend         uintptr
	Fclose        uintptr
	Fwend         uintptr
	Fwpos         uintptr
	Fmustbezero_1 uintptr
	Fwbase        uintptr
	Fread         uintptr
	Fwrite        uintptr
	Fseek         uintptr
	Fbuf          uintptr
	Fbuf_size     uint32
	Fprev         uintptr
	Fnext         uintptr
	F_fileno      int32
	Fpipe_pid     int32
	Flockcount    int32
	Fmode         int32
	Flock         int32
	Flbf          int32
	Fcookie       uintptr
	Foff          int64
	Fgetln_buf    uintptr
	Fmustbezero_2 uintptr
	Fshend        uintptr
	F__ccgo_pad1  [4]byte
	Fshlim        int64
	Fshcnt        int64
	Fprev_locked  uintptr
	Fnext_locked  uintptr
	Flocale       uintptr
	F__ccgo_pad2  [4]byte
}
