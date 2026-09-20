package gitsync

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/elliot40404/creds/internal/vaultfiles"

	"github.com/elliot40404/creds/internal/crypto"
	"github.com/elliot40404/creds/internal/vault"
)

func mustClone(t *testing.T, url string) *Repo {
	t.Helper()
	r, err := Clone(context.Background(), url, filepath.Join(t.TempDir(), "joined"))
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestCloneVault(t *testing.T) {
	t.Parallel()
	p := newVaultPair(t)
	p.put(t, p.a, secretEntry("e", "1"))
	p.sync(t, p.a, Pushed)
	r := mustClone(t, p.bare)
	if p.password(t, r, "e") != "1" {
		t.Fatal("entry missing")
	}
	if got := gitOut(t, r, "symbolic-ref", "--short", "HEAD"); got != Branch {
		t.Fatalf("branch %q", got)
	}
	if got := gitOut(t, r, "config", "--local", "user.name"); got != r.git.userName() {
		t.Fatalf("user.name %q", got)
	}
	if st := mustStatus(t, r); dirty(t, r) || st.Ahead != 0 || st.Behind != 0 || st.Remote != p.bare {
		t.Fatalf("status %+v", st)
	}
	p.put(t, r, secretEntry("f", "2"))
	p.sync(t, r, Pushed)
	p.sync(t, p.a, FastForwarded)
}

func TestCloneEmptyRemote(t *testing.T) {
	t.Parallel()
	bare := newBare(t)
	r := mustClone(t, bare)
	data, err := os.ReadFile(filepath.Join(r.Dir, vaultfiles.IgnoreFile))
	if err != nil || string(data) != vaultfiles.IgnoreRules {
		t.Fatalf("gitignore %q %v", data, err)
	}
	writeVault(t, r)
	mustCommit(t, r, "init")
	if got := mustSync(t, r); got != Pushed {
		t.Fatalf("sync %v", got)
	}
	if bareHead(t, bare) == "" {
		t.Fatal("remote not updated")
	}
}

func TestCloneRejectsDisallowedTree(t *testing.T) {
	t.Parallel()
	p := newVaultPair(t)
	writeFile(t, p.a, "notes.txt", "x")
	gitOut(t, p.a, "add", "-f", "notes.txt")
	gitOut(t, p.a, "commit", "-q", "-m", "bad")
	gitOut(t, p.a, "push", "-q", Remote, Branch)
	dir := filepath.Join(t.TempDir(), "joined")
	if _, err := Clone(context.Background(), p.bare, dir); !errors.Is(err, ErrNotAllowed) {
		t.Fatalf("want ErrNotAllowed, got %v", err)
	}
	if _, err := os.Lstat(dir); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("dir left behind: %v", err)
	}
}

func TestCloneErrors(t *testing.T) {
	t.Parallel()
	needGit(t)
	ctx := context.Background()
	if _, err := Clone(ctx, "-bad", filepath.Join(t.TempDir(), "x")); !errors.Is(err, ErrBadRemote) {
		t.Fatalf("want ErrBadRemote, got %v", err)
	}
	existing := t.TempDir()
	if _, err := Clone(ctx, newBare(t), existing); !errors.Is(err, fs.ErrExist) {
		t.Fatalf("want ErrExist, got %v", err)
	}
	missing := filepath.Join(t.TempDir(), "gone.git")
	dir := filepath.Join(t.TempDir(), "x")
	if _, err := Clone(ctx, missing, dir); err == nil {
		t.Fatal("clone of missing remote succeeded")
	}
	if _, err := os.Lstat(dir); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("dir left behind: %v", err)
	}
}

func TestCloneThenLoadNeedsIdentity(t *testing.T) {
	t.Parallel()
	p := newVaultPair(t)
	r := mustClone(t, p.bare)
	other, err := crypto.NewIdentity()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := vault.Load(r.Dir, other); !errors.Is(err, vault.ErrRecipientMismatch) {
		t.Fatalf("want ErrRecipientMismatch, got %v", err)
	}
}

func TestCloneRefusesOtherTransports(t *testing.T) {
	t.Parallel()
	for _, url := range []string{"ext::sh -c touch% pwned", "fd::3", "evil::x", "git://127.0.0.1:1/x", "http://127.0.0.1:1/x"} {
		dir := filepath.Join(t.TempDir(), "joined")
		_, err := Clone(context.Background(), url, dir)
		if err == nil || !strings.Contains(err.Error(), "not allowed") {
			t.Fatalf("%s: want transport refused, got %v", url, err)
		}
	}
}
