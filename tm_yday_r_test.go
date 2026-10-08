// Copyright 2026 The Libc Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// localtime_r is a stub on illumos and windows.

//go:build !illumos && !windows

package libc // import "modernc.org/libc"

import "testing"

func TestLocaltimeRYday(t *testing.T) {
	tls := NewTLS()
	defer tls.Close()

	tm := Xcalloc(tls, 1, 128)
	defer Xfree(tls, tm)

	testYdayCheck(t, "localtime_r", func(tls *TLS, tp uintptr) uintptr { return Xlocaltime_r(tls, tp, tm) })
}
