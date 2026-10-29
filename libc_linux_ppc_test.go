// Copyright 2026 The Libc Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package libc

import (
	"os"
	"path/filepath"
	"testing"
	"time"
	"unsafe"

	"golang.org/x/sys/unix"
	"modernc.org/libc/fcntl"
	"modernc.org/libc/stdio"
	"modernc.org/libc/sys/stat"
	"modernc.org/libc/sys/types"
	ctime "modernc.org/libc/time"
)

// The C side of these layouts was measured by Adelie gcc 13.3.0 in the Adelie
// 1.0-beta6 ppc rootfs: runs/libc-linuxppc-20261029/capture/layout.out in the
// port workspace.
func TestLinuxPPCLayouts(t *testing.T) {
	var st stat.Stat
	for _, tc := range []struct {
		name      string
		got, want uintptr
	}{
		{"sizeof(struct stat)", unsafe.Sizeof(st), 152},
		{"st_ino", unsafe.Offsetof(st.Fst_ino), 8},
		{"st_mode", unsafe.Offsetof(st.Fst_mode), 16},
		{"st_rdev", unsafe.Offsetof(st.Fst_rdev), 32},
		{"st_size", unsafe.Offsetof(st.Fst_size), 48},
		{"st_blksize", unsafe.Offsetof(st.Fst_blksize), 56},
		{"st_blocks", unsafe.Offsetof(st.Fst_blocks), 64},
		{"st_atim", unsafe.Offsetof(st.Fst_atim), 104},
		{"st_mtim", unsafe.Offsetof(st.Fst_mtim), 120},
		{"st_ctim", unsafe.Offsetof(st.Fst_ctim), 136},
		{"sizeof(struct timespec)", unsafe.Sizeof(ctime.Timespec{}), 16},
		{"timespec.tv_nsec", unsafe.Offsetof(ctime.Timespec{}.Ftv_nsec), 12},
		{"sizeof(time_t)", unsafe.Sizeof(types.Time_t(0)), 8},
		{"sizeof(off_t)", unsafe.Sizeof(types.Off_t(0)), 8},
		// stdio.FILE must be the translated musl _IO_FILE, field for field.
		{"sizeof(FILE)", unsafe.Sizeof(stdio.FILE{}), unsafe.Sizeof(_IO_FILE{})},
		{"FILE.flags", unsafe.Offsetof(stdio.FILE{}.F_flags), unsafe.Offsetof(_IO_FILE{}.Fflags)},
		{"FILE.fd", unsafe.Offsetof(stdio.FILE{}.F_fileno), unsafe.Offsetof(_IO_FILE{}.Ffd)},
		{"FILE.lock", unsafe.Offsetof(stdio.FILE{}.Flock), unsafe.Offsetof(_IO_FILE{}.Flock)},
		{"FILE.locale", unsafe.Offsetof(stdio.FILE{}.Flocale), unsafe.Offsetof(_IO_FILE{}.Flocale)},
	} {
		if tc.got != tc.want {
			t.Errorf("%s = %d, want %d", tc.name, tc.got, tc.want)
		}
	}
}

func linuxPPCCString(tls *TLS, s string) uintptr {
	p := Xmalloc(tls, types.Size_t(len(s)+1))
	copy(unsafe.Slice((*byte)(unsafe.Pointer(p)), len(s)+1), s+"\x00")
	return p
}

func linuxPPCErrno(tls *TLS) int32 { return *(*int32)(unsafe.Pointer(X__errno_location(tls))) }

// linuxPPCAlloc returns C memory for a T. Passing &goStackVar as a uintptr is
// not safe: the goroutine stack can move during the call, and the callee then
// writes to the old copy, which is what translated C never does.
func linuxPPCAlloc[T any](tls *TLS) (*T, func()) {
	var zero T
	p := Xcalloc(tls, 1, types.Size_t(unsafe.Sizeof(zero)))
	return (*T)(unsafe.Pointer(p)), func() { Xfree(tls, p) }
}

// TestLinuxPPCFileIO drives the calls sqlite's unix VFS makes, through libc's
// X functions, and checks each result against an independent channel (Go's
// os package, which goes through the runtime's own syscalls).
func TestLinuxPPCFileIO(t *testing.T) {
	tls := NewTLS()
	defer tls.Close()

	path := filepath.Join(t.TempDir(), "f")
	cpath := linuxPPCCString(tls, path)
	defer Xfree(tls, cpath)
	va := NewVaList(int32(0640))
	defer Xfree(tls, va)
	fd := Xopen(tls, cpath, fcntl.O_RDWR|fcntl.O_CREAT, va)
	if fd < 0 {
		t.Fatalf("open: errno %d", linuxPPCErrno(tls))
	}
	defer Xclose(tls, fd)

	buf := Xmalloc(tls, 16)
	defer Xfree(tls, buf)
	copy(unsafe.Slice((*byte)(unsafe.Pointer(buf)), 5), "hello")
	if n := Xwrite(tls, fd, buf, 5); n != 5 {
		t.Fatalf("write: %d errno %d", n, linuxPPCErrno(tls))
	}

	// pwrite/pread beyond 4 GiB: the halves of the 64-bit offset are visible.
	const far int64 = 5<<30 + 7
	if n := Xpwrite(tls, fd, buf, 5, far); n != 5 {
		t.Fatalf("pwrite(%d): %d errno %d", far, n, linuxPPCErrno(tls))
	}
	if fi, err := os.Stat(path); err != nil || fi.Size() != far+5 {
		t.Fatalf("after pwrite(%d): size %v, %v; want %d", far, fi.Size(), err, far+5)
	}
	clear(unsafe.Slice((*byte)(unsafe.Pointer(buf)), 16))
	if n := Xpread(tls, fd, buf, 5, far); n != 5 || GoString(buf) != "hello" {
		t.Fatalf("pread(%d): %d %q", far, n, GoString(buf))
	}
	if g := Xlseek64(tls, fd, 0, 2 /* SEEK_END */); g != far+5 {
		t.Fatalf("lseek(SEEK_END) = %d, want %d", g, far+5)
	}
	if Xftruncate64(tls, fd, 4096) != 0 {
		t.Fatalf("ftruncate: errno %d", linuxPPCErrno(tls))
	}

	// fstat/stat fill musl's struct stat, including the time64 fields.
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	stp, free := linuxPPCAlloc[stat.Stat](tls)
	defer free()
	for _, name := range []string{"fstat", "stat", "lstat"} {
		*stp = stat.Stat{}
		var rc int32
		switch name {
		case "fstat":
			rc = Xfstat(tls, fd, uintptr(unsafe.Pointer(stp)))
		case "stat":
			rc = Xstat(tls, cpath, uintptr(unsafe.Pointer(stp)))
		case "lstat":
			rc = Xlstat(tls, cpath, uintptr(unsafe.Pointer(stp)))
		}
		st := *stp
		if rc != 0 {
			t.Fatalf("%s: errno %d", name, linuxPPCErrno(tls))
		}
		if st.Fst_size != 4096 || st.Fst_mode&0170000 != 0100000 || st.Fst_mode&0777 != 0640 {
			t.Errorf("%s: size %d mode %#o", name, st.Fst_size, st.Fst_mode)
		}
		mt := time.Unix(st.Fst_mtim.Ftv_sec, int64(st.Fst_mtim.Ftv_nsec))
		if !mt.Equal(fi.ModTime()) {
			t.Errorf("%s: st_mtim %v, os.Stat %v", name, mt, fi.ModTime())
		}
		if st.Fst_atim.Ftv_sec == 0 || st.Fst_ctim.Ftv_sec == 0 {
			t.Errorf("%s: st_atim %d st_ctim %d not filled", name, st.Fst_atim.Ftv_sec, st.Fst_ctim.Ftv_sec)
		}
	}

	// fcntl F_SETFD/F_GETFD with an int vararg, as sqlite's robust_open does.
	va2 := NewVaList(int32(fcntl.FD_CLOEXEC))
	defer Xfree(tls, va2)
	if Xfcntl(tls, fd, fcntl.F_SETFD, va2) != 0 {
		t.Fatalf("F_SETFD: errno %d", linuxPPCErrno(tls))
	}
	if g := Xfcntl(tls, fd, fcntl.F_GETFD, 0); g != fcntl.FD_CLOEXEC {
		t.Errorf("F_GETFD = %d, want FD_CLOEXEC", g)
	}
	if got, err := unix.FcntlInt(uintptr(fd), unix.F_GETFD, 0); err != nil || got != unix.FD_CLOEXEC {
		t.Errorf("kernel F_GETFD = %d, %v", got, err)
	}

	// fcntl F_SETLK/F_GETLK with a struct flock pointer vararg.
	lk, freeLk := linuxPPCAlloc[fcntl.Flock](tls)
	defer freeLk()
	*lk = fcntl.Flock{Fl_type: fcntl.F_WRLCK, Fl_whence: 0, Fl_start: far, Fl_len: 1}
	va3 := NewVaList(uintptr(unsafe.Pointer(lk)))
	defer Xfree(tls, va3)
	if Xfcntl(tls, fd, fcntl.F_SETLK, va3) != 0 {
		t.Fatalf("F_SETLK: errno %d", linuxPPCErrno(tls))
	}

	if Xunlink(tls, cpath) != 0 {
		t.Fatalf("unlink: errno %d", linuxPPCErrno(tls))
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("after unlink: %v", err)
	}
}

func TestLinuxPPCTime(t *testing.T) {
	tls := NewTLS()
	defer tls.Close()

	now := time.Now()
	ttp, freeTt := linuxPPCAlloc[types.Time_t](tls)
	defer freeTt()
	r := Xtime(tls, uintptr(unsafe.Pointer(ttp)))
	tt := *ttp
	if d := int64(tt) - now.Unix(); r != tt || d < -5 || d > 5 {
		t.Errorf("time() = %d, *tloc = %d, Go %d", r, tt, now.Unix())
	}
	tvp, freeTv := linuxPPCAlloc[types.Timeval](tls)
	defer freeTv()
	if Xgettimeofday(tls, uintptr(unsafe.Pointer(tvp)), 0) != 0 {
		t.Fatalf("gettimeofday: errno %d", linuxPPCErrno(tls))
	}
	tv := *tvp
	if d := tv.Ftv_sec - now.Unix(); d < -5 || d > 5 || tv.Ftv_usec < 0 || tv.Ftv_usec >= 1e6 {
		t.Errorf("gettimeofday = %d.%06d, Go %d", tv.Ftv_sec, tv.Ftv_usec, now.Unix())
	}
	tmp, freeTm := linuxPPCAlloc[ctime.Tm](tls)
	defer freeTm()
	if Xlocaltime_r(tls, uintptr(unsafe.Pointer(ttp)), uintptr(unsafe.Pointer(tmp))) == 0 {
		t.Fatal("localtime_r returned NULL")
	}
	tm := *tmp
	if want := time.Unix(int64(tt), 0).In(getLocalLocation()); tm.Ftm_year != int32(want.Year()-1900) || tm.Ftm_mday != int32(want.Day()) {
		t.Errorf("localtime_r: year %d mday %d, want %d %d", tm.Ftm_year+1900, tm.Ftm_mday, want.Year(), want.Day())
	}
}

func TestLinuxPPCRlimit(t *testing.T) {
	tls := NewTLS()
	defer tls.Close()

	var want unix.Rlimit
	if err := unix.Prlimit(0, unix.RLIMIT_NOFILE, nil, &want); err != nil {
		t.Fatal(err)
	}
	gotp, free := linuxPPCAlloc[[2]uint64](tls) // musl struct rlimit: two 64-bit rlim_t
	defer free()
	if Xgetrlimit(tls, unix.RLIMIT_NOFILE, uintptr(unsafe.Pointer(gotp))) != 0 {
		t.Fatalf("getrlimit: errno %d", linuxPPCErrno(tls))
	}
	got := *gotp
	if got[0] != want.Cur || got[1] != want.Max {
		t.Errorf("getrlimit(NOFILE) = %v, prlimit64 %v/%v", got, want.Cur, want.Max)
	}
}
