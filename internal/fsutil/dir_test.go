package fsutil

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestEnsureDir(t *testing.T) {
	path := filepath.Join(t.TempDir(), "a", "b")
	if err := EnsureDir(path); err != nil {
		t.Fatal(err)
	}
	if err := EnsureDir(path); err != nil {
		t.Fatalf("second call: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if !info.IsDir() {
		t.Fatal("not a dir")
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != DirPerm {
		t.Fatalf("perm %o, want %o", info.Mode().Perm(), DirPerm)
	}
}

func TestEnsureDirFileInWay(t *testing.T) {
	path := filepath.Join(t.TempDir(), "f")
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := EnsureDir(path); err == nil {
		t.Fatal("want error")
	}
}

func TestCheckAbsent(t *testing.T) {
	dir := t.TempDir()
	if err := CheckAbsent(filepath.Join(dir, "missing")); err != nil {
		t.Fatalf("missing: %v", err)
	}
	if err := CheckAbsent(dir); !errors.Is(err, fs.ErrExist) {
		t.Fatalf("existing: %v", err)
	}
}
