package app

import (
	"bytes"
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/elliot40404/creds/internal/crypto"
	"github.com/elliot40404/creds/internal/gitsync"
	"github.com/elliot40404/creds/internal/vaultfiles"
)

func gitLog(t *testing.T, s *Service) []string {
	t.Helper()
	out, err := gitsync.NewGit(s.Paths.Vault()).Run(context.Background(), "log", "--format=%s%n%b")
	if err != nil {
		t.Fatalf("git log: %v", err)
	}
	return strings.Fields(strings.ReplaceAll(string(out), "update vault", "@"))
}

func TestWritesCommit(t *testing.T) {
	t.Parallel()
	s, fp, _, _ := initVault(t)
	if got := gitLog(t, s); len(got) != 1 {
		t.Fatalf("after init = %v", got)
	}
	if _, err := s.Add(dbEntry("prod/db")); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Update("prod/db", dbEntry("prod/db2")); err != nil {
		t.Fatal(err)
	}
	if err := s.Delete("prod/db2"); err != nil {
		t.Fatal(err)
	}
	fp.passwords = []string{mainWord, nextWord, nextWord}
	if err := s.Passwd(); err != nil {
		t.Fatal(err)
	}
	got := gitLog(t, s)
	if len(got) != 5 {
		t.Fatalf("commits = %v", got)
	}
	for _, m := range got {
		if m != "@" {
			t.Fatalf("unexpected message part %q", m)
		}
	}
}

func TestCommitCleanTree(t *testing.T) {
	t.Parallel()
	s, _, _, _ := initVault(t)
	out, err := gitsync.NewGit(s.Paths.Vault()).Run(context.Background(), "status", "--porcelain", "--untracked-files=all")
	if err != nil || len(out) != 0 {
		t.Fatalf("tree dirty after init: %q %v", out, err)
	}
}

func TestWriteRefusesToSignATamperedCommit(t *testing.T) {
	t.Parallel()
	s, _, _, _ := initVault(t)
	id, err := s.identity()
	if err != nil {
		t.Fatal(err)
	}
	swapped, err := crypto.Seal(id.Recipient(), []byte("older wrapper"))
	if err != nil {
		t.Fatal(err)
	}
	writeHostile(t, s.vaultFile(vaultfiles.PasswordFile), swapped)
	ctx := context.Background()
	g := gitsync.NewGit(s.Paths.Vault())
	for _, args := range [][]string{
		{"add", "--", vaultfiles.PasswordFile},
		{"-c", "user.name=x", "-c", "user.email=x@x", "commit", "-q", "-m", "tamper"},
	} {
		if _, err := g.Run(ctx, args...); err != nil {
			t.Fatal(err)
		}
	}
	before, err := os.ReadFile(s.vaultFile(vaultfiles.ManifestFile))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Add(dbEntry("prod/db")); !errors.Is(err, gitsync.ErrUnverified) {
		t.Fatalf("want unverified, got %v", err)
	}
	after, err := os.ReadFile(s.vaultFile(vaultfiles.ManifestFile))
	if err != nil || !bytes.Equal(after, before) {
		t.Fatalf("manifest re-signed: %v", err)
	}
}
