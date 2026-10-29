//go:build ignore
// +build ignore

package main

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Run explicitly with: go test genheaders.go genheaders_test.go.
func TestLibcHeadersDryRunDoesNotWrite(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	oldOS, oldArch, stdout := goos, goarch, os.Stdout
	defer func() {
		goos, goarch, os.Stdout = oldOS, oldArch, stdout
		if err := os.Chdir(wd); err != nil {
			t.Fatal(err)
		}
	}()
	root := t.TempDir()
	headers := filepath.Join(root, "headers")
	for _, dir := range []string{"dirent", "sys/types", "headers/sys"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0700); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"dirent.h", "sys/types.h"} {
		if err := ioutil.WriteFile(filepath.Join(headers, name), nil, 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	for _, arch := range []string{"ppc", "ppc64"} {
		t.Run(arch, func(t *testing.T) {
			goos, goarch = "darwin", arch
			out, err := ioutil.TempFile(t.TempDir(), "dry-run")
			if err != nil {
				t.Fatal(err)
			}
			defer out.Close()
			os.Stdout = out
			defer func() { os.Stdout = stdout }()
			if err := libcHeaders([]string{headers}, true); err != nil {
				t.Fatal(err)
			}
			data, err := ioutil.ReadFile(out.Name())
			if err != nil {
				t.Fatal(err)
			}
			want := "dirent: dirent_darwin_" + arch + ".go capi_darwin_" + arch + ".go\n" +
				"sys/types: types_darwin_" + arch + ".go capi_darwin_" + arch + ".go\n"
			if string(data) != want {
				t.Fatalf("dry-run output = %q, want %q", data, want)
			}
			if err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}
				if !info.IsDir() && !strings.HasSuffix(path, ".h") {
					t.Errorf("dry-run wrote %s", path)
				}
				return nil
			}); err != nil {
				t.Fatal(err)
			}
		})
	}
}
