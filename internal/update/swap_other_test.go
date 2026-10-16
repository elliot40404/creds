//go:build !windows

package update

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

func TestSwapRenamesOver(t *testing.T) {
	t.Parallel()
	exe := tempExe(t, "creds")
	bin := filepath.Join(filepath.Dir(exe), ".creds-new-test")
	if err := os.WriteFile(bin, []byte(newBin), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := swap(bin, exe); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, exe); got != newBin {
		t.Fatalf("exe %q", got)
	}
	if _, err := os.Stat(bin); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("temp kept: %v", err)
	}
	Cleanup()
}
