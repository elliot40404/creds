package fsutil

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

func openRoot(t *testing.T, dir string) *os.Root {
	t.Helper()
	r, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = r.Close() })
	return r
}

func TestReadFileLimit(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "f")
	if err := os.WriteFile(path, []byte("12345"), FilePerm); err != nil {
		t.Fatal(err)
	}
	if got, err := ReadFileLimit(path, 5); err != nil || string(got) != "12345" {
		t.Fatalf("got %q %v", got, err)
	}
	if _, err := ReadFileLimit(path, 4); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("want ErrTooLarge, got %v", err)
	}
	if _, err := ReadFileLimit(filepath.Join(dir, "missing"), 4); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("want ErrNotExist, got %v", err)
	}
}

func TestReadRegularRejectsDirAndSymlink(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "d"), DirPerm); err != nil {
		t.Fatal(err)
	}
	r := openRoot(t, dir)
	if _, err := ReadRegular(r, "d", 10); !errors.Is(err, errNotRegular) {
		t.Fatalf("dir: %v", err)
	}
	if err := os.Symlink("d", filepath.Join(dir, "l")); err != nil {
		t.Skip("symlinks unavailable:", err)
	}
	if _, err := ReadRegular(r, "l", 10); !errors.Is(err, errNotRegular) {
		t.Fatalf("symlink: %v", err)
	}
}

func TestWriteFileIn(t *testing.T) {
	dir := t.TempDir()
	r := openRoot(t, dir)
	name := filepath.Join("a", "b.enc")
	if err := WriteFileIn(r, name, []byte("x")); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, filepath.Join(dir, name)); string(got) != "x" {
		t.Fatalf("got %q", got)
	}
	assertOnlyFile(t, filepath.Join(dir, "a"), "b.enc")
	if err := WriteFileIn(r, filepath.Join("..", "out"), []byte("x")); err == nil {
		t.Fatal("escape allowed")
	}
}

func TestWriteFileInRefusesSymlinkEscape(t *testing.T) {
	dir := t.TempDir()
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(dir, "x")); err != nil {
		t.Skip("symlinks unavailable:", err)
	}
	r := openRoot(t, dir)
	if err := WriteFileIn(r, filepath.Join("x", "pwned"), []byte("x")); err == nil {
		t.Fatal("write through symlink allowed")
	}
	if _, err := os.Lstat(filepath.Join(outside, "pwned")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("file written outside: %v", err)
	}
}
