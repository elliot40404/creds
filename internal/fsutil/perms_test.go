package fsutil

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCheckPermsOwnerOnly(t *testing.T) {
	path := filepath.Join(t.TempDir(), "f")
	if err := WriteFileAtomic(path, []byte("x")); err != nil {
		t.Fatal(err)
	}
	warn, err := CheckPerms(path)
	if err != nil {
		t.Fatal(err)
	}
	if warn != "" {
		t.Fatalf("unexpected warn %q", warn)
	}
}

func TestCheckPermsMissing(t *testing.T) {
	_, err := CheckPerms(filepath.Join(t.TempDir(), "nope"))
	if !os.IsNotExist(err) {
		t.Fatalf("err %v, want not exist", err)
	}
}
