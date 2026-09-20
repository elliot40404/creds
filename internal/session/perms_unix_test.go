//go:build unix

package session

import (
	"errors"
	"os"
	"testing"
)

func TestLoadDropsLooseSession(t *testing.T) {
	s, _ := newStore(t)
	if err := s.Save("AGE-SECRET-KEY-X"); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(s.Path, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Load(); !errors.Is(err, ErrUnprotected) {
		t.Fatalf("err = %v", err)
	}
	if _, err := os.Stat(s.Path); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("loose session kept")
	}
}
