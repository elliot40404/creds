package session

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/elliot40404/creds/internal/fsutil"
)

func TestLoadRefusesSymlink(t *testing.T) {
	s, _ := newStore(t)
	if err := s.Save("AGE-SECRET-KEY-X"); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(t.TempDir(), "target")
	if err := os.WriteFile(target, []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(s.Path); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, s.Path); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}
	if _, err := s.Load(); !errors.Is(err, ErrNoSession) {
		t.Fatalf("err = %v", err)
	}
	data, err := os.ReadFile(filepath.Clean(target))
	if err != nil || !bytes.Equal(data, []byte("secret")) {
		t.Fatalf("symlink target touched: %v %q", err, data)
	}
}

func TestLoadRefusesOversizeSession(t *testing.T) {
	s, _ := newStore(t)
	if err := s.Save("AGE-SECRET-KEY-X"); err != nil {
		t.Fatal(err)
	}
	big := bytes.Repeat([]byte("a"), maxSessionSize+1)
	if err := fsutil.WriteFileAtomic(s.Path, big); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Load(); !errors.Is(err, ErrNoSession) {
		t.Fatalf("err = %v", err)
	}
	if _, err := os.Stat(s.Path); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("oversize session kept")
	}
}
