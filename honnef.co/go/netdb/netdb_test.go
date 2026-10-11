// Copyright 2026 The Libc Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package netdb // import "modernc.org/libc/honnef.co/go/netdb"

import (
	"sync"
	"testing"
)

// The table sizes as package initialization left them. This init runs after
// those of netdb.go, and before any test can trigger a lookup.
var initProtocols, initServices int

func init() { initProtocols, initServices = len(Protocols), len(Services) }

// TestLoadOnFirstUse checks that the tables stay empty until the first lookup
// and that concurrent first lookups all see the loaded tables.
func TestLoadOnFirstUse(t *testing.T) {
	if initProtocols != 0 || initServices != 0 {
		t.Fatalf("tables loaded before any lookup: %d protocols, %d services", initProtocols, initServices)
	}

	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			tcp := GetProtoByName("tcp")
			if tcp == nil || tcp.Number != 6 {
				t.Errorf("GetProtoByName(tcp) = %+v, want number 6", tcp)
				return
			}
			if s := GetServByName("http", tcp); s == nil || s.Port != 80 {
				t.Errorf("GetServByName(http, tcp) = %+v, want port 80", s)
			}
		})
	}
	wg.Wait()

	if len(Protocols) == 0 || len(Services) == 0 {
		t.Fatalf("tables empty after a lookup: %d protocols, %d services", len(Protocols), len(Services))
	}
}

// TestLoadFallsBack checks that a file that exists but cannot be read, here a
// directory, falls back to the built-in table instead of panicking.
func TestLoadFallsBack(t *testing.T) {
	loadOnce.Do(load) // So that the lookups below do not load again.
	p, s, pf, sf := Protocols, Services, protocolsFile, servicesFile
	defer func() { Protocols, Services, protocolsFile, servicesFile = p, s, pf, sf }()

	Protocols, Services = nil, nil
	protocolsFile, servicesFile = t.TempDir(), t.TempDir()
	load()

	if tcp := GetProtoByNumber(6); tcp == nil || tcp.Name != "tcp" {
		t.Fatalf("GetProtoByNumber(6) = %+v, want tcp from the built-in table", tcp)
	}
	if http := GetServByPort(80, GetProtoByName("tcp")); http == nil || http.Name != "http" {
		t.Fatalf("GetServByPort(80, tcp) = %+v, want http from the built-in table", http)
	}
}
