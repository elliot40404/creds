package gitsync

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/elliot40404/creds/internal/vaultfiles"
)

func newRepo(t *testing.T) *Repo {
	t.Helper()
	needGit(t)
	r, err := InitRepo(context.Background(), filepath.Join(t.TempDir(), "vault"))
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func gitOut(t *testing.T, r *Repo, args ...string) string {
	t.Helper()
	out, err := r.git.Run(context.Background(), args...)
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(string(out))
}

func TestInitRepo(t *testing.T) {
	t.Parallel()
	r := newRepo(t)
	if got := gitOut(t, r, "symbolic-ref", "--short", "HEAD"); got != Branch {
		t.Fatalf("branch %q", got)
	}
	if got := gitOut(t, r, "config", "--local", "user.name"); got != r.git.userName() {
		t.Fatalf("user.name %q", got)
	}
	data, err := os.ReadFile(filepath.Join(r.Dir, vaultfiles.IgnoreFile))
	if err != nil || string(data) != vaultfiles.IgnoreRules {
		t.Fatalf("gitignore %q %v", data, err)
	}
	if _, err := InitRepo(context.Background(), r.Dir); err != nil {
		t.Fatalf("reinit: %v", err)
	}
}

func TestInitRepoIgnoreRules(t *testing.T) {
	t.Parallel()
	r := newRepo(t)
	cases := map[string]bool{
		".gitignore":            false,
		"vault.json":            false,
		"identity.pw.age":       false,
		"identity.recovery.age": false,
		"entries/00.enc":        false,
		"notes.txt":             true,
		"session":               true,
		".vault.json.tmp-1":     true,
		"entries/00.txt":        true,
		"entries/sub/00.enc":    true,
		"other/00.enc":          true,
	}
	for path, want := range cases {
		_, err := r.git.Run(context.Background(), "check-ignore", "-q", "--no-index", path)
		if got := err == nil; got != want {
			t.Errorf("%s ignored=%v want %v (%v)", path, got, want, err)
		}
	}
}
