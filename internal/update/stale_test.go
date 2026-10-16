package update

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"
)

func TestRemoveStaleKeepsFreshAndOthers(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	now := time.Now()
	for name, age := range map[string]time.Duration{
		".creds-download-old": 2 * time.Hour,
		".creds-new-old":      2 * time.Hour,
		".creds-download-new": time.Minute,
		".creds-new-new":      time.Minute,
		"creds.exe":           2 * time.Hour,
		".other-old":          2 * time.Hour,
	} {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(p, now.Add(-age), now.Add(-age)); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(dir, ".creds-new-dir"), 0o700); err != nil {
		t.Fatal(err)
	}
	RemoveStale(dir, now)
	want := []string{".creds-download-new", ".creds-new-dir", ".creds-new-new"}
	if got := leftovers(t, dir); !slices.Equal(got, want) {
		t.Fatalf("left %v", got)
	}
	for _, name := range []string{"creds.exe", ".other-old"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Fatal(err)
		}
	}
	RemoveStale(filepath.Join(dir, "missing"), now)
}
