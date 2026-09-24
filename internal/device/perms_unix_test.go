//go:build unix

package device

import (
	"errors"
	"os"
	"testing"

	"filippo.io/age/plugin"
)

func TestOpenDropsLooseFile(t *testing.T) {
	s, _, id := trusted(t)
	if err := os.Chmod(s.Path, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Open(vaultOf(id), &plugin.ClientUI{}); !errors.Is(err, ErrUnprotected) {
		t.Fatalf("got %v", err)
	}
	assertGone(t, s)
}
