//go:build unix

package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func checkSSHPerms(t *testing.T, dir, rel string) {
	t.Helper()
	want := map[string]os.FileMode{dir: 0o700, filepath.Join(dir, filepath.FromSlash(rel)): 0o600}
	for p := filepath.Dir(rel); p != "."; p = filepath.Dir(p) {
		want[filepath.Join(dir, filepath.FromSlash(p))] = 0o700
	}
	for p, mode := range want {
		info, err := os.Lstat(p)
		if err != nil {
			t.Fatal(err)
		}
		if got := info.Mode().Perm(); got != mode {
			t.Errorf("%s: mode %o, want %o", strings.TrimPrefix(p, dir), got, mode)
		}
	}
}
