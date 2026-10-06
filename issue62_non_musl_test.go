// Copyright 2026 The Libc Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !windows && !(linux && (amd64 || arm64 || loong64 || ppc64le || s390x || riscv64 || 386 || arm))

package libc // import "modernc.org/libc"

import (
	"time"
	"unsafe"

	"modernc.org/libc/errno"
	"modernc.org/libc/pthread"
	ctime "modernc.org/libc/time"
)

const (
	issue62Normal    int32 = pthread.PTHREAD_MUTEX_NORMAL
	issue62Recursive int32 = pthread.PTHREAD_MUTEX_RECURSIVE
	issue62ETIMEDOUT int32 = errno.ETIMEDOUT
)

// issue62Deadline sets the timespec at ts to now+d on CLOCK_REALTIME.
func issue62Deadline(ts uintptr, d time.Duration) {
	t := time.Now().Add(d)
	p := (*ctime.Timespec)(unsafe.Pointer(ts))
	issue62SetInt(&p.Ftv_sec, t.Unix())
	issue62SetInt(&p.Ftv_nsec, int64(t.Nanosecond()))
}
