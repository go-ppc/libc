// Copyright 2026 The Libc Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !(linux && (amd64 || arm64 || loong64 || ppc64le || s390x || riscv64 || 386 || arm))

package libc // import "modernc.org/libc"

import (
	"bytes"
	"math/rand"
	"strconv"
	"strings"
	"testing"
	"unsafe"
)

// Reference implementations: the byte loops replaced by the current code.

func refStrchr(s uintptr, c int32) uintptr {
	for {
		ch2 := *(*byte)(unsafe.Pointer(s))
		if ch2 == byte(c) {
			return s
		}
		if ch2 == 0 {
			return 0
		}
		s++
	}
}

func refStrcpy(dest, src uintptr) uintptr {
	r := dest
	for ; ; dest++ {
		c := *(*int8)(unsafe.Pointer(src))
		src++
		*(*int8)(unsafe.Pointer(dest)) = c
		if c == 0 {
			return r
		}
	}
}

func refStrncpy(dest, src uintptr, n Tsize_t) uintptr {
	r := dest
	for c := *(*int8)(unsafe.Pointer(src)); c != 0 && n > 0; n-- {
		*(*int8)(unsafe.Pointer(dest)) = c
		dest++
		src++
		c = *(*int8)(unsafe.Pointer(src))
	}
	for ; uintptr(n) > 0; n-- {
		*(*int8)(unsafe.Pointer(dest)) = 0
		dest++
	}
	return r
}

func refStrncat(dest, src uintptr, n Tsize_t) uintptr {
	r := dest
	for *(*byte)(unsafe.Pointer(dest)) != 0 {
		dest++
	}
	for ; n != 0; n-- {
		c := *(*byte)(unsafe.Pointer(src))
		if c == 0 {
			break
		}
		*(*byte)(unsafe.Pointer(dest)) = c
		dest++
		src++
	}
	*(*byte)(unsafe.Pointer(dest)) = 0
	return r
}

func refStrstr(haystack, needle uintptr) uintptr {
	if i := strings.Index(GoString(haystack), GoString(needle)); i >= 0 {
		return haystack + uintptr(i)
	}
	return 0
}

// cbuf is a C heap buffer holding a C string at offset off; the
// remaining bytes are fill. Go memory would trip checkptr under -race.
type cbuf struct {
	b   []byte
	off int
}

func newCbuf(tb testing.TB, tls *TLS, s string, off, pad int, fill byte) cbuf {
	n := off + len(s) + 1 + pad
	p := Xmalloc(tls, Tsize_t(n))
	tb.Cleanup(func() { Xfree(tls, p) })
	b := unsafe.Slice((*byte)(unsafe.Pointer(p)), n)
	for i := range b {
		b[i] = fill
	}
	copy(b[off:], s)
	b[off+len(s)] = 0
	return cbuf{b, off}
}

func (c cbuf) p() uintptr { return uintptr(unsafe.Pointer(&c.b[c.off])) }

func randCString(r *rand.Rand, n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = byte(1 + r.Intn(255))
	}
	return string(b)
}

var strcopyLens = []int{0, 1, 2, 7, 8, 9, 15, 16, 17, 63, 64, 65, 255, 4095, 4096, 4097, 9000}

func TestStrchrDiff(t *testing.T) {
	tls := NewTLS()
	defer tls.Close()
	r := rand.New(rand.NewSource(1))
	cs := []int32{0, 1, 'a', 0x7f, 0x80, 0xff, 0x100, 0x161, -1, -256, -255}
	for _, n := range strcopyLens {
		for off := 0; off < 3; off++ {
			s := []byte(randCString(r, n))
			if n > 0 {
				s[r.Intn(n)] = 'a'
				s[n-1] = 0x61 // 0x161 converts to 'a'
			}
			b := newCbuf(t, tls, string(s), off, 8, 'a') // bytes after the NUL must not match
			for _, c := range cs {
				if g, w := Xstrchr(tls, b.p(), c), refStrchr(b.p(), c); g != w {
					t.Fatalf("n=%d off=%d c=%#x: got %#x want %#x", n, off, c, g-b.p(), w-b.p())
				}
			}
		}
	}
}

func TestStrstrDiff(t *testing.T) {
	tls := NewTLS()
	defer tls.Close()
	r := rand.New(rand.NewSource(2))
	for _, n := range strcopyLens {
		for _, m := range []int{0, 1, 2, 5, 16, 100} {
			for off := 0; off < 3; off++ {
				hs := []byte(randCString(r, n))
				nd := randCString(r, m)
				if m <= n && r.Intn(2) == 0 {
					copy(hs[r.Intn(n-m+1):], nd)
				}
				h := newCbuf(t, tls, string(hs), off, 8, 'x')
				nb := newCbuf(t, tls, nd, 1-off%2, 8, 'y')
				if g, w := Xstrstr(tls, h.p(), nb.p()), refStrstr(h.p(), nb.p()); g != w {
					t.Fatalf("n=%d m=%d off=%d: got %#x want %#x", n, m, off, g, w)
				}
			}
		}
	}
	if g := Xstrstr(tls, 0, 0); g != 0 {
		t.Fatalf("strstr(NULL, NULL) = %#x", g)
	}
}

// checkCopy runs f and ref on identically prepared destinations and
// compares the whole buffers, guard bytes included.
func checkCopy(t *testing.T, tls *TLS, name string, dst string, size, off int, fill byte, f, ref func(d uintptr) uintptr) {
	t.Helper()
	got := newCbuf(t, tls, dst, off, size, fill)
	want := newCbuf(t, tls, dst, off, size, fill)
	if g, w := f(got.p()), ref(want.p()); g-got.p() != w-want.p() {
		t.Fatalf("%s: return %#x, want %#x", name, g-got.p(), w-want.p())
	}
	if !bytes.Equal(got.b, want.b) {
		t.Fatalf("%s: buffers differ\ngot  %q\nwant %q", name, got.b, want.b)
	}
}

func TestStrcopyDiff(t *testing.T) {
	tls := NewTLS()
	defer tls.Close()
	r := rand.New(rand.NewSource(3))
	for _, n := range strcopyLens {
		for off := 0; off < 3; off++ {
			for _, fill := range []byte{0xaa, 0x55, 0xff} {
				src := newCbuf(t, tls, randCString(r, n), off, 8, 'z')
				s := src.p()
				name := "n=" + strconv.Itoa(n) + " off=" + strconv.Itoa(off)
				checkCopy(t, tls, "strcpy "+name, "", n+16, (off+1)%3, fill,
					func(d uintptr) uintptr { return Xstrcpy(tls, d, s) },
					func(d uintptr) uintptr { return refStrcpy(d, s) })
				for _, k := range []int{0, 1, n / 2, n - 1, n, n + 1, n + 13} {
					if k < 0 {
						continue
					}
					kn := Tsize_t(k)
					name := name + " k=" + strconv.Itoa(k)
					checkCopy(t, tls, "strncpy "+name, "", k+16, (off+1)%3, fill,
						func(d uintptr) uintptr { return Xstrncpy(tls, d, s, kn) },
						func(d uintptr) uintptr { return refStrncpy(d, s, kn) })
					for _, pre := range []string{"", "a", "prefix", strings.Repeat("p", 15), strings.Repeat("p", 16), strings.Repeat("p", 40)} {
						checkCopy(t, tls, "strncat "+name+" pre="+pre, pre, min(k, n)+16, (off+2)%3, fill,
							func(d uintptr) uintptr { return strncat(d, s, kn) },
							func(d uintptr) uintptr { return refStrncat(d, s, kn) })
					}
				}
			}
		}
	}
}

// strncpy and strncat must not read the source past n bytes even when
// there is no NUL within them.
func TestStrnlenUnterminated(t *testing.T) {
	tls := NewTLS()
	defer tls.Close()
	src := newCbuf(t, tls, strings.Repeat("q", 8200), 1, 0, 0)
	for _, n := range []int{0, 1, 7, 4095, 4096, 4097, 8199, 8200} {
		if g := strnlen(src.p(), Tsize_t(n)); int(g) != n {
			t.Fatalf("strnlen n=%d: %d", n, g)
		}
	}
	if g := strnlen(src.p(), 9000); g != 8200 {
		t.Fatalf("strnlen n=9000: %d", g)
	}
}

func benchSizes(b *testing.B, f func(b *testing.B, size int)) {
	for _, size := range []int{8, 64, 1024, 65536} {
		b.Run(strconv.Itoa(size), func(b *testing.B) {
			b.SetBytes(int64(size))
			f(b, size)
		})
	}
}

func BenchmarkStrcopy(b *testing.B) {
	tls := NewTLS()
	defer tls.Close()
	for _, impl := range []struct {
		name string
		f    func(b *testing.B, src, dst, nd uintptr, size int)
	}{
		{"strchr/old", func(b *testing.B, src, _, _ uintptr, _ int) {
			for i := 0; i < b.N; i++ {
				refStrchr(src, 'a')
			}
		}},
		{"strchr/new", func(b *testing.B, src, _, _ uintptr, _ int) {
			for i := 0; i < b.N; i++ {
				Xstrchr(tls, src, 'a')
			}
		}},
		{"strcpy/old", func(b *testing.B, src, dst, _ uintptr, _ int) {
			for i := 0; i < b.N; i++ {
				refStrcpy(dst, src)
			}
		}},
		{"strcpy/new", func(b *testing.B, src, dst, _ uintptr, _ int) {
			for i := 0; i < b.N; i++ {
				Xstrcpy(tls, dst, src)
			}
		}},
		{"strncpy/old", func(b *testing.B, src, dst, _ uintptr, size int) {
			for i := 0; i < b.N; i++ {
				refStrncpy(dst, src, Tsize_t(size+8))
			}
		}},
		{"strncpy/new", func(b *testing.B, src, dst, _ uintptr, size int) {
			for i := 0; i < b.N; i++ {
				Xstrncpy(tls, dst, src, Tsize_t(size+8))
			}
		}},
		{"strncat/old", func(b *testing.B, src, dst, _ uintptr, size int) {
			for i := 0; i < b.N; i++ {
				*(*byte)(unsafe.Pointer(dst)) = 0
				refStrncat(dst, src, Tsize_t(size))
			}
		}},
		{"strncat/new", func(b *testing.B, src, dst, _ uintptr, size int) {
			for i := 0; i < b.N; i++ {
				*(*byte)(unsafe.Pointer(dst)) = 0
				strncat(dst, src, Tsize_t(size))
			}
		}},
		{"strstr/old", func(b *testing.B, src, _, nd uintptr, _ int) {
			for i := 0; i < b.N; i++ {
				refStrstr(src, nd)
			}
		}},
		{"strstr/new", func(b *testing.B, src, _, nd uintptr, _ int) {
			for i := 0; i < b.N; i++ {
				Xstrstr(tls, src, nd)
			}
		}},
	} {
		b.Run(impl.name, func(b *testing.B) {
			benchSizes(b, func(b *testing.B, size int) {
				// Match (or needle) at the end: the whole string is scanned.
				s := strings.Repeat("x", size-3) + "abc"
				src := newCbuf(b, tls, s, 1, 8, 0)
				nd := newCbuf(b, tls, "abc", 1, 8, 0)
				dst := newCbuf(b, tls, "", 1, size+16, 0)
				impl.f(b, src.p(), dst.p(), nd.p(), size)
			})
		})
	}
}
