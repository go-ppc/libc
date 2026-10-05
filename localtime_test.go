// Copyright 2026 The Libc Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build darwin || freebsd || netbsd || openbsd

package libc // import "modernc.org/libc"

import (
	"os"
	"testing"
	"unsafe"

	"modernc.org/libc/sys/types"
	"modernc.org/libc/time"
)

func setenvTZ(tls *TLS, name uintptr, tz string) {
	value := mustCString(tz)
	defer Xfree(tls, value)
	Xsetenv(tls, name, value, 1)
}

// withTZ runs f with TZ set in the C environment, unset for "".
func withTZ(tb testing.TB, tz string, f func(tls *TLS, tp, tm uintptr)) {
	tls := NewTLS()
	defer tls.Close()

	name := mustCString("TZ")
	defer Xfree(tls, name)
	if prev, ok := os.LookupEnv("TZ"); ok {
		defer setenvTZ(tls, name, prev)
	} else {
		defer Xunsetenv(tls, name)
	}

	if tz == "" {
		Xunsetenv(tls, name)
	} else {
		setenvTZ(tls, name, tz)
	}

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
		{"Europe/Berlin", 1767268800, 13, 0, 3600},              // 2026-01-01 12:00 UTC
		{"Europe/Berlin", 1782907200, 14, 1, 7200},              // 2026-07-01 12:00 UTC
		{"America/New_York", 1767268800, 7, 0, -18000},          // TZ change is picked up
		{"America/New_York", 1782907200, 8, 1, -14400},          //
		{"Australia/Sydney", 1767268800, 23, 1, 39600},          // southern summer
		{"Asia/Kolkata", 1782907200, 17, 0, 19800},              // no DST, half hour offset
		{"UTC", 1782907200, 12, 0, 0},                           //
		{"CET-1CEST,M3.5.0,M10.5.0/3", 1767268800, 13, 0, 3600}, // not loadable, fixed zone
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
