// Copyright 2026 The Libc Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !libc.membrk

package libc // import "modernc.org/libc"

import "testing"

// tlsCloseBlockSize is big enough for every block the test allocates to get a
// mapping of its own from the allocator, see TestTLSCloseLiveFrames.
const tlsCloseBlockSize = 20000

// TestTLSCloseLiveFrames checks that Close frees everything a TLS owns when it
// is closed with frames still live on it: a panic recovered outside the frames
// skipped the Free and FreeAlloca calls they would have made. The frames span
// several stack segments and one segment was left empty before the panic, so
// the walk has to cover the segments before and after the current one.
//
// A leak is detected through the allocator. A block big enough to get a mapping
// of its own is retained for reuse once freed, and later allocations of its
// size class get the retained mappings back, newest first, before a new one is
// mapped. A block that leaked stays live and is never handed out again. Under
// libc.memgrind the audit also reports every leaked block directly.
func TestTLSCloseLiveFrames(t *testing.T) {
	NewTLS().Close() // allocations the first TLS of a process makes and keeps
	aux := NewTLS()  // for the allocations that check the result

	defer aux.Close()

	if err := trimAllocator(); err != nil { // so that everything freed below is retained for reuse
		t.Fatal(err)
	}

	MemAuditStart() // a leak audit under libc.memgrind, a no-op otherwise
	tls := NewTLS()
	var blocks []uintptr // what Close must free
	func() {
		defer func() { recover() }()

		for i := 0; i < 2; i++ {
			tls.Alloc(tlsCloseBlockSize)
			blocks = append(blocks, tlsStackBlock(tls))
			tlsAllocaEnter(tls) // the exit is what the panic skips
			blocks = append(blocks, Xalloca(tls, tlsCloseBlockSize))
		}
		tls.Alloc(tlsCloseBlockSize)
		blocks = append(blocks, tlsStackBlock(tls))
		tls.Free(tlsCloseBlockSize) // an empty segment now follows the current one
		panic("skipping the frees of two frames")
	}()
	tls.Close()
	if err := MemAuditReport(); err != nil {
		t.Fatal(err)
	}

	// A couple more than len(blocks): freeing the small blocks a TLS owns can
	// retire the shared pages they lived in, which then come back first.
	got := map[uintptr]bool{}
	var again []uintptr
	for i := 0; i < len(blocks)+2; i++ {
		p := Xmalloc(aux, tlsCloseBlockSize)
		if p == 0 {
			t.Fatal("out of memory")
		}

		again = append(again, p)
		got[p] = true
	}
	for _, p := range again {
		Xfree(aux, p)
	}
	for i, p := range blocks {
		if !got[p] {
			t.Errorf("block %d at %#x was not freed by Close", i, p)
		}
	}
}
