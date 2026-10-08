// Copyright 2026 The Libc Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// localtime is a stub on illumos.

//go:build !illumos

package libc // import "modernc.org/libc"

import (
	"testing"
	"unsafe"
)

// testTmFields returns tm_sec through tm_yday, the eight leading ints that
// struct tm has in the same order on every port.
func testTmFields(tm uintptr) (f [8]int32) {
	return *(*[8]int32)(unsafe.Pointer(tm))
}

// testYdayCheck calls localtime for noon UTC of every day from 2023-12-30 to
// 2025-01-02 and checks tm_yday against the date in the same struct tm, so the
// result does not depend on TZ. 2024 is a leap year, so tm_yday must reach 365.
func testYdayCheck(t *testing.T, name string, localtime func(tls *TLS, tp uintptr) uintptr) {
	tls := NewTLS()
	defer tls.Close()

	tp := Xcalloc(tls, 1, 8)
	defer Xfree(tls, tp)

	var daysBefore = [2][12]int32{
		{0, 31, 59, 90, 120, 151, 181, 212, 243, 273, 304, 334},
		{0, 31, 60, 91, 121, 152, 182, 213, 244, 274, 305, 335},
	}
	seen0, seen365 := false, false
	for ts := int64(1703937600); ts <= int64(1735819200); ts += 86400 { // 2023-12-30 to 2025-01-02, 12:00 UTC
		*(*int64)(unsafe.Pointer(tp)) = ts // time_t is 32 bits on freebsd/386, little endian there
		f := testTmFields(localtime(tls, tp))
		mday, mon, year, yday := f[3], f[4], f[5]+1900, f[7]
		leap := 0
		if year%4 == 0 && (year%100 != 0 || year%400 == 0) {
			leap = 1
		}
		if want := daysBefore[leap][mon] + mday - 1; yday != want {
			t.Fatalf("%s(%d): %04d-%02d-%02d has tm_yday %d, want %d", name, ts, year, mon+1, mday, yday, want)
		}
		seen0 = seen0 || yday == 0
		seen365 = seen365 || yday == 365
	}
	if !seen0 || !seen365 {
		t.Fatalf("%s: tm_yday 0 seen %v, 365 seen %v", name, seen0, seen365)
	}
}

func TestLocaltimeYday(t *testing.T) {
	testYdayCheck(t, "localtime", func(tls *TLS, tp uintptr) uintptr { return Xlocaltime(tls, tp) })
}
