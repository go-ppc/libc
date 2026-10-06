// Copyright 2026 The Libc Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package libc // import "modernc.org/libc"

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unsafe"

	"golang.org/x/sys/unix"
	"modernc.org/libc/stdio"
)

func stdioTestFile(t *testing.T, content, mode string) (string, *TLS, uintptr) {
	fn := filepath.Join(t.TempDir(), "f")
	if err := os.WriteFile(fn, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	tls := NewTLS()
	t.Cleanup(tls.Close)
	f := Xfopen(tls, mustCString(fn), mustCString(mode))
	if f == 0 {
		t.Fatal("fopen")
	}

	return fn, tls, f
}

func stdioFgets(tls *TLS, f uintptr, size int32) (string, bool) {
	buf := Xmalloc(tls, Tsize_t(size))
	defer Xfree(tls, buf)
	if Xfgets(tls, buf, size, f) == 0 {
		return "", false
	}

	return GoString(buf), true
}

func stdioOffset(t *testing.T, tls *TLS, f uintptr) int64 {
	n, err := unix.Seek(int(Xfileno(tls, f)), 0, 1)
	if err != nil {
		t.Fatal(err)
	}

	return n
}

func TestStdioFgets(t *testing.T) {
	_, tls, f := stdioTestFile(t, "a\nbb\n\nhello\nccc", "r")
	defer Xfclose(tls, f)

	for _, e := range []string{"a\n", "bb\n", "\n", "hel", "lo\n", "ccc"} {
		size := int32(16)
		if strings.HasPrefix(e, "hel") || e == "lo\n" {
			size = 4
		}
		if g, ok := stdioFgets(tls, f, size); !ok || g != e {
			t.Fatalf("got %q %v, expected %q", g, ok, e)
		}
	}
	if Xfeof(tls, f) == 0 {
		t.Fatal("expected EOF after unterminated last line")
	}
	if g, ok := stdioFgets(tls, f, 16); ok {
		t.Fatalf("got %q, expected NULL at EOF", g)
	}
	if g, ok := stdioFgets(tls, f, 1); !ok || g != "" {
		t.Fatalf("size 1: got %q %v", g, ok)
	}
}

func TestStdioFgetc(t *testing.T) {
	_, tls, f := stdioTestFile(t, "xy", "r")
	defer Xfclose(tls, f)

	for _, e := range []int32{'x', 'y', stdio.EOF, stdio.EOF} {
		if g := Xfgetc(tls, f); g != e {
			t.Fatalf("got %v, expected %v", g, e)
		}
	}
	if Xfeof(tls, f) == 0 || Xferror(tls, f) != 0 {
		t.Fatal("expected EOF, no error")
	}
	Xclearerr(tls, f)
	if Xfeof(tls, f) != 0 {
		t.Fatal("clearerr did not clear EOF")
	}
}

func TestStdioFread(t *testing.T) {
	var sb strings.Builder
	for i := 0; sb.Len() < 3*fileBufSize; i++ {
		sb.WriteByte(byte(i))
	}
	content := sb.String()
	_, tls, f := stdioTestFile(t, content, "r")
	defer Xfclose(tls, f)

	buf := Xmalloc(tls, Tsize_t(len(content)+10))
	defer Xfree(tls, buf)
	b := unsafe.Slice((*byte)(unsafe.Pointer(buf)), len(content)+10)
	off := 0
	for _, n := range []int{1, 100, 2 * fileBufSize, 50} {
		if g := Xfread(tls, buf+uintptr(off), 1, Tsize_t(n), f); g != Tsize_t(n) {
			t.Fatalf("fread %v: got %v", n, g)
		}
		off += n
		if g := Xftell(tls, f); g != long(off) {
			t.Fatalf("ftell: got %v, expected %v", g, off)
		}
	}
	rest := len(content) - off
	if g := Xfread(tls, buf+uintptr(off), 1, Tsize_t(rest+10), f); g != Tsize_t(rest) {
		t.Fatalf("short fread: got %v, expected %v", g, rest)
	}
	if string(b[:len(content)]) != content {
		t.Fatal("content mismatch")
	}
	if Xfeof(tls, f) == 0 {
		t.Fatal("expected EOF")
	}
}

func TestStdioUngetc(t *testing.T) {
	_, tls, f := stdioTestFile(t, "abc\n", "r")
	defer Xfclose(tls, f)

	// Before any read.
	if g := Xungetc(tls, 'Z', f); g != 'Z' {
		t.Fatalf("ungetc: got %v", g)
	}
	if g := Xfgetc(tls, f); g != 'Z' {
		t.Fatalf("got %v, expected Z", g)
	}
	if g := Xfgetc(tls, f); g != 'a' {
		t.Fatalf("got %v, expected a", g)
	}
	if g := Xungetc(tls, 'X', f); g != 'X' {
		t.Fatalf("ungetc: got %v", g)
	}
	if g := Xftell(tls, f); g != 0 {
		t.Fatalf("ftell after ungetc: got %v, expected 0", g)
	}
	if g, _ := stdioFgets(tls, f, 16); g != "Xbc\n" {
		t.Fatalf("got %q", g)
	}
	if Xfgetc(tls, f) != stdio.EOF || Xfeof(tls, f) == 0 {
		t.Fatal("expected EOF")
	}
	if Xungetc(tls, 'Q', f) != 'Q' || Xfeof(tls, f) != 0 {
		t.Fatal("ungetc must clear EOF")
	}
	if g := Xfgetc(tls, f); g != 'Q' {
		t.Fatalf("got %v, expected Q", g)
	}
	if Xungetc(tls, stdio.EOF, f) != stdio.EOF {
		t.Fatal("ungetc(EOF) must fail")
	}
}

func TestStdioFtellFseek(t *testing.T) {
	const content = "0123456789\nabcdef\n"
	_, tls, f := stdioTestFile(t, content, "r")
	defer Xfclose(tls, f)

	if g, _ := stdioFgets(tls, f, 6); g != "01234" {
		t.Fatalf("got %q", g)
	}
	if g := Xftell(tls, f); g != 5 {
		t.Fatalf("ftell: got %v, expected 5", g)
	}
	// Fully buffered input reads ahead, like glibc and musl.
	if g := stdioOffset(t, tls, f); g != int64(len(content)) {
		t.Fatalf("descriptor offset: got %v, expected %v", g, len(content))
	}
	if Xfseek(tls, f, 2, stdio.SEEK_CUR) != 0 {
		t.Fatal("fseek")
	}
	if g := Xfgetc(tls, f); g != '7' {
		t.Fatalf("got %q, expected 7", g)
	}
	if Xfseek(tls, f, 11, stdio.SEEK_SET) != 0 {
		t.Fatal("fseek")
	}
	if g, _ := stdioFgets(tls, f, 16); g != "abcdef\n" {
		t.Fatalf("got %q", g)
	}
	Xfgetc(tls, f)
	if Xfeof(tls, f) == 0 {
		t.Fatal("expected EOF")
	}
	Xrewind(tls, f)
	if Xfeof(tls, f) != 0 || Xfgetc(tls, f) != '0' {
		t.Fatal("rewind")
	}
	// fflush on an input stream syncs the descriptor offset.
	Xfflush(tls, f)
	if g := stdioOffset(t, tls, f); g != 1 {
		t.Fatalf("descriptor offset after fflush: got %v, expected 1", g)
	}
	if g := Xfgetc(tls, f); g != '1' {
		t.Fatalf("got %q, expected 1", g)
	}
}

func TestStdioReadWrite(t *testing.T) {
	fn, tls, f := stdioTestFile(t, "line1\nline2\n", "r+")
	if g, _ := stdioFgets(tls, f, 16); g != "line1\n" {
		t.Fatalf("got %q", g)
	}
	// C requires a positioning call when switching from reading to writing.
	if Xfseek(tls, f, 0, stdio.SEEK_CUR) != 0 {
		t.Fatal("fseek")
	}
	if Xfputs(tls, mustCString("LINE"), f) < 0 {
		t.Fatal("fputs")
	}
	if Xfseek(tls, f, 0, stdio.SEEK_SET) != 0 {
		t.Fatal("fseek")
	}
	if g, _ := stdioFgets(tls, f, 16); g != "line1\n" {
		t.Fatalf("got %q", g)
	}
	if g, _ := stdioFgets(tls, f, 16); g != "LINE2\n" {
		t.Fatalf("got %q", g)
	}
	if Xfclose(tls, f) != 0 {
		t.Fatal("fclose")
	}
	if b, _ := os.ReadFile(fn); string(b) != "line1\nLINE2\n" {
		t.Fatalf("file: %q", b)
	}
}

func TestStdioWrite(t *testing.T) {
	fn, tls, f := stdioTestFile(t, "", "w")
	Xfputs(tls, mustCString("abc"), f)
	Xfputc(tls, '\n', f)
	// Output is unbuffered, stderr included.
	if b, _ := os.ReadFile(fn); string(b) != "abc\n" {
		t.Fatalf("before fflush: %q", b)
	}
	data := mustCString("0123456789")
	Xfwrite(tls, data, 1, 10, f)
	Xfflush(tls, f)
	if b, _ := os.ReadFile(fn); string(b) != "abc\n0123456789" {
		t.Fatalf("after fflush: %q", b)
	}
	Xfwrite(tls, data, 2, 2, f)
	if Xfclose(tls, f) != 0 {
		t.Fatal("fclose")
	}
	if b, _ := os.ReadFile(fn); string(b) != "abc\n01234567890123" {
		t.Fatalf("after fclose: %q", b)
	}
}

func TestStdioUnbuffered(t *testing.T) {
	_, tls, f := stdioTestFile(t, "abc", "r")
	defer Xfclose(tls, f)

	if Xsetvbuf(tls, f, 0, stdio.X_IONBF, 0) != 0 {
		t.Fatal("setvbuf")
	}
	if g := Xfgetc(tls, f); g != 'a' {
		t.Fatalf("got %v", g)
	}
	if g := stdioOffset(t, tls, f); g != 1 {
		t.Fatalf("unbuffered stream read ahead: offset %v", g)
	}
	if g, _ := stdioFgets(tls, f, 16); g != "bc" {
		t.Fatalf("got %q", g)
	}
}

// A pipe delivers what is available, fgets must not wait for a full buffer.
func TestStdioPipe(t *testing.T) {
	var p [2]int
	if err := unix.Pipe(p[:]); err != nil {
		t.Fatal(err)
	}

	defer unix.Close(p[1])
	tls := NewTLS()
	defer tls.Close()
	f := Xfdopen(tls, int32(p[0]), mustCString("r"))
	defer Xfclose(tls, f)

	unix.Write(p[1], []byte("x\ny\n"))
	for _, e := range []string{"x\n", "y\n"} {
		if g, _ := stdioFgets(tls, f, 16); g != e {
			t.Fatalf("got %q, expected %q", g, e)
		}
	}
	if Xftell(tls, f) != -1 {
		t.Fatal("ftell on a pipe must fail")
	}
}

// Emulates the darwin __sgetc macro: --_r < 0 ? __srget(p) : *_p++.
func TestStdioSrget(t *testing.T) {
	content := strings.Repeat("0123456789", fileBufSize/5)
	_, tls, f := stdioTestFile(t, content, "r")
	defer Xfclose(tls, f)

	s := file(f).s()
	var b []byte
	for {
		var c int32
		if s.F_r--; s.F_r < 0 {
			c = X__srget(tls, f)
		} else {
			c = int32(*(*byte)(unsafe.Pointer(s.F_p)))
			s.F_p++
		}
		if c == stdio.EOF {
			break
		}

		b = append(b, byte(c))
	}
	if string(b) != content {
		t.Fatal("content mismatch")
	}
}

func stdioBenchFile(b *testing.B, content, mode string) (*TLS, uintptr) {
	fn := filepath.Join(b.TempDir(), "f")
	if err := os.WriteFile(fn, []byte(content), 0o600); err != nil {
		b.Fatal(err)
	}

	tls := NewTLS()
	b.Cleanup(tls.Close)
	f := Xfopen(tls, mustCString(fn), mustCString(mode))
	if f == 0 {
		b.Fatal("fopen")
	}

	b.Cleanup(func() { Xfclose(tls, f) })
	return tls, f
}

func BenchmarkStdioFgets64(b *testing.B) {
	tls, f := stdioBenchFile(b, strings.Repeat(strings.Repeat("x", 63)+"\n", 1024), "r")
	buf := Xmalloc(tls, 128)
	b.SetBytes(64)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if Xfgets(tls, buf, 128, f) == 0 {
			Xfseek(tls, f, 0, stdio.SEEK_SET)
			Xclearerr(tls, f)
		}
	}
}

func BenchmarkStdioFgetc(b *testing.B) {
	tls, f := stdioBenchFile(b, strings.Repeat("x", 1<<16), "r")
	b.SetBytes(1)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if Xfgetc(tls, f) == stdio.EOF {
			Xfseek(tls, f, 0, stdio.SEEK_SET)
			Xclearerr(tls, f)
		}
	}
}

func BenchmarkStdioFread4K(b *testing.B) {
	tls, f := stdioBenchFile(b, strings.Repeat("x", 1<<20), "r")
	buf := Xmalloc(tls, 4096)
	b.SetBytes(4096)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if Xfread(tls, buf, 1, 4096, f) != 4096 {
			Xfseek(tls, f, 0, stdio.SEEK_SET)
			Xclearerr(tls, f)
		}
	}
}

func BenchmarkStdioFputs64(b *testing.B) {
	tls, f := stdioBenchFile(b, "", "w")
	s := mustCString(strings.Repeat("x", 63) + "\n")
	b.SetBytes(64)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Xfputs(tls, s, f)
	}
}

func BenchmarkStdioFwrite4K(b *testing.B) {
	tls, f := stdioBenchFile(b, "", "w")
	buf := Xcalloc(tls, 1, 4096)
	b.SetBytes(4096)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Xfwrite(tls, buf, 1, 4096, f)
	}
}
