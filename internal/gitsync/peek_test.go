package gitsync

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/elliot40404/creds/internal/vaultfiles"
)

func TestPeekRejectsBadURL(t *testing.T) {
	t.Parallel()
	for _, url := range []string{"", "--upload-pack=touch", "http://x\nrm"} {
		if _, err := Peek(context.Background(), url); !errors.Is(err, ErrBadRemote) {
			t.Fatalf("%q: err = %v", url, err)
		}
	}
}

func TestPeekEmptyRemote(t *testing.T) {
	t.Parallel()
	dir := t.TempDir() + "/remote.git"
	if _, err := NewGit(t.TempDir()).Run(context.Background(), "init", "-q", "--bare", "-b", Branch, dir); err != nil {
		t.Fatal(err)
	}
	names, err := Peek(context.Background(), dir)
	if err != nil {
		t.Fatalf("peek: %v", err)
	}
	if len(names) != 0 {
		t.Fatalf("names = %v", names)
	}
}

func TestPeekIgnoresSharedTempDir(t *testing.T) {
	tmp := t.TempDir()
	for _, k := range []string{"TMPDIR", "TMP", "TEMP"} {
		t.Setenv(k, tmp)
	}
	if err := os.WriteFile(filepath.Join(tmp, ".git"), []byte("gitdir: "+filepath.Join(tmp, "missing")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(t.TempDir(), "remote.git")
	if _, err := NewGit(t.TempDir()).Run(context.Background(), "init", "-q", "--bare", "-b", Branch, dir); err != nil {
		t.Fatal(err)
	}
	if _, err := Peek(context.Background(), dir); err != nil {
		t.Fatalf("peek: %v", err)
	}
}

func TestPeekListsVaultFiles(t *testing.T) {
	t.Parallel()
	p := newVaultPair(t)
	p.put(t, p.a, secretEntry("e", "1"))
	p.sync(t, p.a, Pushed)
	names, err := Peek(context.Background(), p.bare)
	if err != nil {
		t.Fatalf("peek: %v", err)
	}
	if !slices.Contains(names, vaultfiles.MetaFile) {
		t.Fatalf("names = %v", names)
	}
}
