// Copyright 2026 The Libc Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build darwin || freebsd || netbsd || openbsd

package libc // import "modernc.org/libc"

import (
	"strings"
	"testing"
	"unsafe"

	"modernc.org/libc/sys/types"
	"modernc.org/libc/time"
)

// withTZ runs f with TZ set in the C environment, unset for "". It swaps in an environ
// array of its own instead of calling setenv and unsetenv, which not every port running
// this test implements.
func withTZ(tb testing.TB, tz string, f func(tls *TLS, tp, tm uintptr)) {
	tls := NewTLS()
	defer tls.Close()

	const ptrSize = unsafe.Sizeof(uintptr(0))
	old := Environ()
	var entries []uintptr
	for p := old; *(*uintptr)(unsafe.Pointer(p)) != 0; p += ptrSize {
		if e := *(*uintptr)(unsafe.Pointer(p)); !strings.HasPrefix(GoString(e), "TZ=") {
			entries = append(entries, e)
		}
	}
	var tzEntry uintptr
	if tz != "" {
		tzEntry = mustCString("TZ=" + tz)
		entries = append(entries, tzEntry)
	}
	env := Xcalloc(tls, types.Size_t(len(entries)+1), types.Size_t(ptrSize))
	for i, e := range entries {
		*(*uintptr)(unsafe.Pointer(env + uintptr(i)*ptrSize)) = e
	}
	Xenviron = env
	defer func() {
		Xenviron = old
		Xfree(tls, env)
		Xfree(tls, tzEntry)
	}()

	tp := Xmalloc(tls, 8)
	tm := Xmalloc(tls, types.Size_t(unsafe.Sizeof(time.Tm{})))
	defer Xfree(tls, tp)
	defer Xfree(tls, tm)

	f(tls, tp, tm)
}

func TestLocaltimeTZ(t *testing.T) {
	for _, test := range []struct {
		tz     string
		ts     int64
		hour   int32
		isdst  int32
		gmtoff int64
	}{
		{"Europe/Berlin", 1767268800, 13, 0, 3600},                 // 2026-01-01 12:00 UTC
		{"Europe/Berlin", 1782907200, 14, 1, 7200},                 // 2026-07-01 12:00 UTC
		{"America/New_York", 1767268800, 7, 0, -18000},             // TZ change is picked up
		{"America/New_York", 1782907200, 8, 1, -14400},             //
		{"Australia/Sydney", 1767268800, 23, 1, 39600},             // southern summer
		{"Asia/Kolkata", 1782907200, 17, 0, 19800},                 // no DST, half hour offset
		{"UTC", 1782907200, 12, 0, 0},                              //
		{"CET-1CEST,M3.5.0,M10.5.0/3", 1767268800, 13, 0, 3600},    // POSIX rule
		{"CET-1CEST,M3.5.0,M10.5.0/3", 1782907200, 14, 1, 7200},    // its DST used to be ignored
		{"EST5EDT,M3.2.0,M11.1.0", 1772953199, 1, 0, -18000},       // 1 s before 2026-03-08 02:00 EST
		{"EST5EDT,M3.2.0,M11.1.0", 1772953200, 3, 1, -14400},       // DST starts
		{"EST5EDT,M3.2.0,M11.1.0", 1782907200, 8, 1, -14400},       //
		{"AEST-10AEDT,M10.1.0,M4.1.0/3", 1767268800, 23, 1, 39600}, // southern summer
		{"AEST-10AEDT,M10.1.0,M4.1.0/3", 1782907200, 22, 0, 36000}, //
		{"<+03>-3", 1782907200, 15, 0, 10800},                      // quoted name, no DST
		{"IST-5:30", 1782907200, 17, 0, 19800},                     // offset with minutes
		{"America/Port-au-Prince", 1767268800, 7, 0, -18000},       // '-' in the name used to exit the process
		{"Asia/Ust-Nera", 1782907200, 22, 0, 36000},                //
		{"GB-Eire", 1782907200, 13, 1, 3600},                       //
		{":Europe/Berlin", 1782907200, 14, 1, 7200},                // POSIX allows a colon before a name
		{":No/Such_Zone", 1782907200, 12, 0, 0},                    // colon name that does not load: UTC
		{":", 1782907200, 12, 0, 0},                                //
	} {
		withTZ(t, test.tz, func(tls *TLS, tp, tm uintptr) {
			*(*int64)(unsafe.Pointer(tp)) = test.ts
			Xlocaltime_r(tls, tp, tm)

			r := (*time.Tm)(unsafe.Pointer(tm))
			if r.Ftm_hour != test.hour || r.Ftm_isdst != test.isdst || int64(r.Ftm_gmtoff) != test.gmtoff {
				t.Errorf("%s %d: got hour %d isdst %d gmtoff %d, expected %d %d %d",
					test.tz, test.ts, r.Ftm_hour, r.Ftm_isdst, r.Ftm_gmtoff, test.hour, test.isdst, test.gmtoff)
			}
		})
	}
}

func TestMktimePosixTZ(t *testing.T) {
	for _, test := range []struct {
		tz   string
		mon  int32
		want int64
	}{
		{"EST5EDT,M3.2.0,M11.1.0", 6, 1782921600}, // 2026-07-01 12:00 EDT
		{"EST5EDT,M3.2.0,M11.1.0", 0, 1767286800}, // 2026-01-01 12:00 EST
	} {
		withTZ(t, test.tz, func(tls *TLS, tp, tm uintptr) {
			*(*time.Tm)(unsafe.Pointer(tm)) = time.Tm{Ftm_year: 126, Ftm_mon: test.mon, Ftm_mday: 1, Ftm_hour: 12}
			if got := int64(Xmktime(tls, tm)); got != test.want {
				t.Errorf("%s month %d: mktime %d, want %d", test.tz, test.mon+1, got, test.want)
			}
		})
	}
}

func BenchmarkLocaltime(b *testing.B) {
	for _, tz := range []string{"", "Europe/Berlin"} {
		b.Run("TZ="+tz, func(b *testing.B) {
			withTZ(b, tz, func(tls *TLS, tp, tm uintptr) {
				ts := int64(1767268800)
				for b.Loop() {
					*(*int64)(unsafe.Pointer(tp)) = ts
					Xlocaltime_r(tls, tp, tm)
					ts += 900
				}
			})
		})
	}
}
