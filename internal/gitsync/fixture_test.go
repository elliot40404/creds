package gitsync

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/elliot40404/creds/internal/crypto"
	"github.com/elliot40404/creds/internal/testutil"
)

const (
	partBare    = "remote.git"
	partA       = "vault"
	partB       = "clone"
	placeholder = "creds-fixture-bare"
)

var vaultFixtureID *crypto.Identity

func copiedPair(t *testing.T, name string, build func() (string, *Repo, *Repo)) (string, *Repo, *Repo) {
	t.Helper()
	needGit(t)
	src := fixtures.Dir(t, name, func(dir string) {
		bare, a, b := build()
		testutil.CopyTree(t, bare, filepath.Join(dir, partBare))
		dropHooks(t, filepath.Join(dir, partBare))
		for part, r := range map[string]*Repo{partA: a, partB: b} {
			testutil.CopyTree(t, r.Dir, filepath.Join(dir, part))
			swapURL(t, filepath.Join(dir, part), bare, placeholder)
			dropHooks(t, filepath.Join(dir, part, ".git"))
		}
	})
	bare := copyPart(t, src, partBare)
	a, b := Open(copyPart(t, src, partA)), Open(copyPart(t, src, partB))
	swapURL(t, a.Dir, placeholder, bare)
	swapURL(t, b.Dir, placeholder, bare)
	return bare, a, b
}

func copyPart(t *testing.T, src, part string) string {
	t.Helper()
	dst := filepath.Join(t.TempDir(), part)
	testutil.CopyTree(t, filepath.Join(src, part), dst)
	return dst
}

func swapURL(t *testing.T, dir, from, to string) {
	t.Helper()
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = root.Close() }()
	const path = ".git/config"
	data, err := root.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	old := []byte("url = " + configQuote(from) + "\n")
	if !bytes.Contains(data, old) {
		t.Fatalf("%s has no url %q", dir, from)
	}
	data = bytes.Replace(data, old, []byte("url = "+configQuote(to)+"\n"), 1)
	if err := root.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func dropHooks(t *testing.T, gitDir string) {
	t.Helper()
	if err := os.RemoveAll(filepath.Join(gitDir, "hooks")); err != nil {
		t.Fatal(err)
	}
}

func configQuote(s string) string {
	return strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(s)
}
