package fsutil

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func readFile(t *testing.T, path string) []byte {
	t.Helper()
	got, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func assertOnlyFile(t *testing.T, dir, name string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != name {
		names := make([]string, 0, len(entries))
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Fatalf("dir entries %v, want only %s", names, name)
	}
}

func TestWriteFileAtomicNew(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "f.enc")
	if err := WriteFileAtomic(path, []byte("hello")); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, path); !bytes.Equal(got, []byte("hello")) {
		t.Fatalf("got %q", got)
	}
	assertOnlyFile(t, dir, "f.enc")
}

func TestWriteFileAtomicOverwrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "f.enc")
	if err := os.WriteFile(path, []byte("old content"), FilePerm); err != nil {
		t.Fatal(err)
	}
	if err := WriteFileAtomic(path, []byte("new")); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, path); !bytes.Equal(got, []byte("new")) {
		t.Fatalf("got %q", got)
	}
	assertOnlyFile(t, dir, "f.enc")
}

func TestWriteFileAtomicPerm(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix perm bits")
	}
	path := filepath.Join(t.TempDir(), "f")
	if err := WriteFileAtomic(path, []byte("x")); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != FilePerm {
		t.Fatalf("perm %o, want %o", info.Mode().Perm(), FilePerm)
	}
}

func TestWriteFileAtomicFailureKeepsOld(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "f.enc")
	if err := os.WriteFile(path, []byte("old"), FilePerm); err != nil {
		t.Fatal(err)
	}
	errBoom := errors.New("boom")
	renameIn = func(*os.Root, string, string) error { return errBoom }
	t.Cleanup(func() { renameIn = (*os.Root).Rename })
	if err := WriteFileAtomic(path, []byte("new")); !errors.Is(err, errBoom) {
		t.Fatalf("err %v, want boom", err)
	}
	if got := readFile(t, path); !bytes.Equal(got, []byte("old")) {
		t.Fatalf("got %q", got)
	}
	assertOnlyFile(t, dir, "f.enc")
}

func TestWriteFileAtomicMissingDir(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nope", "f.enc")
	if err := WriteFileAtomic(path, []byte("x")); err == nil {
		t.Fatal("want error")
	}
}
