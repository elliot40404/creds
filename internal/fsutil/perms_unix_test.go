//go:build unix

package fsutil

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCheckPermsUnix(t *testing.T) {
	dir := t.TempDir()
	cases := []struct {
		name string
		dir  bool
		perm os.FileMode
		warn bool
	}{
		{"file600", false, 0o600, false},
		{"file400", false, 0o400, false},
		{"file640", false, 0o640, true},
		{"file604", false, 0o604, true},
		{"dir700", true, 0o700, false},
		{"dir750", true, 0o750, true},
		{"dir701", true, 0o701, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(dir, tc.name)
			if tc.dir {
				if err := os.Mkdir(path, 0o700); err != nil {
					t.Fatal(err)
				}
			} else if err := os.WriteFile(path, nil, 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.Chmod(path, tc.perm); err != nil {
				t.Fatal(err)
			}
			warn, err := CheckPerms(path)
			if err != nil {
				t.Fatal(err)
			}
			if (warn != "") != tc.warn {
				t.Fatalf("warn %q, want warn=%v", warn, tc.warn)
			}
			if tc.warn && !strings.Contains(warn, path) {
				t.Fatalf("warn %q missing path", warn)
			}
		})
	}
}

func TestEnsureDirTightensExistingDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "home")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := EnsureDir(dir); err != nil {
		t.Fatal(err)
	}
	if warn, err := CheckPerms(dir); err != nil || warn != "" {
		t.Fatalf("warn %q err %v", warn, err)
	}
}

func TestCheckPermsFlagsAnotherOwner(t *testing.T) {
	path := filepath.Join(t.TempDir(), "f")
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	me := currentUID
	currentUID = func() int { return os.Getuid() + 1 }
	defer func() { currentUID = me }()
	warn, err := CheckPerms(path)
	if err != nil || !strings.Contains(warn, "owned by") {
		t.Fatalf("warn %q err %v", warn, err)
	}
}
