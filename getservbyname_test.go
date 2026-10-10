// Copyright 2026 The Libc Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build darwin || freebsd || netbsd || openbsd || illumos

package libc // import "modernc.org/libc"

import (
	"testing"
	"unsafe"

	"modernc.org/libc/netdb"
)

func TestGetservbyname(t *testing.T) {
	tls := NewTLS()
	defer tls.Close()

	get := func(name, proto string) *netdb.Servent {
		n := mustCString(name)
		defer Xfree(tls, n)
		var p uintptr
		if proto != "" {
			p = mustCString(proto)
			defer Xfree(tls, p)
		}
		return (*netdb.Servent)(unsafe.Pointer(Xgetservbyname(tls, n, p)))
	}
	// The port is in network byte order.
	check := func(s *netdb.Servent, name, proto string) {
		t.Helper()
		if s == nil {
			t.Fatalf("getservbyname(%q, %q) = NULL", name, proto)
		}

		port := uint16(s.Fs_port)
		if g := *(*[2]byte)(unsafe.Pointer(&port)); g != [2]byte{0, 80} {
			t.Fatalf("getservbyname(%q, %q): s_port bytes %v, want [0 80]", name, proto, g)
		}

		if g := GoString(s.Fs_proto); proto != "" && g != proto || g == "" {
			t.Fatalf("getservbyname(%q, %q): s_proto %q", name, proto, g)
		}

		// The services files differ: on OpenBSD http is an alias of www, NetBSD
		// lists both as names without aliases.
		names := []string{GoString(s.Fs_name)}
		for p := s.Fs_aliases; *(*uintptr)(unsafe.Pointer(p)) != 0; p += unsafe.Sizeof(uintptr(0)) {
			names = append(names, GoString(*(*uintptr)(unsafe.Pointer(p))))
		}
		for _, v := range names {
			if v == name {
				return
			}
		}

		t.Fatalf("getservbyname(%q, %q): name and aliases %q", name, proto, names)
	}

	// Repeated calls free the previous result.
	for i := 0; i < 10; i++ {
		check(get("http", "tcp"), "http", "tcp")
		check(get("www", "tcp"), "www", "tcp")
		// A NULL proto matches any protocol.
		check(get("http", ""), "http", "")
	}
	if s := get("http", "no-such-protocol"); s != nil {
		t.Fatalf("unknown protocol: %q", GoString(s.Fs_name))
	}

	if s := get("no-such-service", "tcp"); s != nil {
		t.Fatalf("unknown service: %q", GoString(s.Fs_name))
	}
}
