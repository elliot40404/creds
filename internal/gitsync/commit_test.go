package gitsync

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/elliot40404/creds/internal/vaultfiles"
)

const ageBlob = "age-encryption.org/v1\n-> X25519 stub\n"

func writeFile(t *testing.T, r *Repo, name, data string) {
	t.Helper()
	path := filepath.Join(r.Dir, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
}

func writeVault(t *testing.T, r *Repo) {
	t.Helper()
	writeFile(t, r, "vault.json", `{"format_version":3}`)
	writeFile(t, r, "identity.pw.age", ageBlob)
	writeFile(t, r, "identity.recovery.age", ageBlob)
	for slot := range vaultfiles.BucketCount {
		writeFile(t, r, "entries/"+vaultfiles.BucketName(slot), ageBlob)
	}
	writeFile(t, r, vaultfiles.ManifestFile, ageBlob)
}

func mustCommit(t *testing.T, r *Repo, msg string) bool {
	t.Helper()
	ok, err := r.Commit(context.Background(), msg)
	if err != nil {
		t.Fatal(err)
	}
	return ok
}

func TestCommitAllowlisted(t *testing.T) {
	t.Parallel()
	r := newRepo(t)
	writeVault(t, r)
	writeFile(t, r, "notes.txt", "secret")
	writeFile(t, r, "entries/x.txt", "secret")
	if !mustCommit(t, r, "init") {
		t.Fatal("nothing committed")
	}
	files := gitOut(t, r, "ls-tree", "-r", "--name-only", "HEAD")
	want := strings.Join(slices.Sorted(slices.Values(vaultfiles.Required())), "\n")
	if files != want {
		t.Fatalf("tree %q", files)
	}
	if mustCommit(t, r, "again") {
		t.Fatal("no-op commit created")
	}
	if n := gitOut(t, r, "rev-list", "--count", "HEAD"); n != "1" {
		t.Fatalf("commits %s", n)
	}
}

func TestCommitDeletion(t *testing.T) {
	t.Parallel()
	r := newRepo(t)
	writeVault(t, r)
	mustCommit(t, r, "init")
	if err := os.Remove(filepath.Join(r.Dir, "identity.pw.age")); err != nil {
		t.Fatal(err)
	}
	if !mustCommit(t, r, "rm") {
		t.Fatal("deletion not committed")
	}
	if files := gitOut(t, r, "ls-tree", "-r", "--name-only", "HEAD"); strings.Contains(files, "identity.pw.age") {
		t.Fatalf("tree %q", files)
	}
}

func TestCommitRejectsPlaintext(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"entries/00.enc", "identity.recovery.age"} {
		t.Run(name, func(t *testing.T) {
			r := newRepo(t)
			writeVault(t, r)
			writeFile(t, r, name, "plain secret")
			_, err := r.Commit(context.Background(), "bad")
			if !errors.Is(err, ErrPlaintext) {
				t.Fatalf("want ErrPlaintext, got %v", err)
			}
			if _, err := r.git.Run(context.Background(), "rev-parse", "--verify", "-q", "HEAD"); err == nil {
				t.Fatal("commit created")
			}
			assertNoObject(t, r, "plain secret")
		})
	}
}

func TestCommitRejectsEmptyEncrypted(t *testing.T) {
	t.Parallel()
	r := newRepo(t)
	writeFile(t, r, "entries/01.enc", "")
	if _, err := r.Commit(context.Background(), "bad"); !errors.Is(err, ErrPlaintext) {
		t.Fatalf("want ErrPlaintext, got %v", err)
	}
}

func TestCommitRejectsForcedPath(t *testing.T) {
	t.Parallel()
	r := newRepo(t)
	writeVault(t, r)
	writeFile(t, r, "notes.txt", "secret")
	gitOut(t, r, "add", "-f", "notes.txt")
	_, err := r.Commit(context.Background(), "bad")
	if !errors.Is(err, ErrNotAllowed) {
		t.Fatalf("want ErrNotAllowed, got %v", err)
	}
}

func assertNoObject(t *testing.T, r *Repo, data string) {
	t.Helper()
	sha := hashBlob(t, r, data)
	if _, err := r.git.Run(context.Background(), "cat-file", "-e", sha); err == nil {
		t.Fatalf("object %s written", sha)
	}
	if staged := gitOut(t, r, "ls-files", "-s"); strings.Contains(staged, sha) {
		t.Fatalf("index holds %s", sha)
	}
}

func hashBlob(t *testing.T, r *Repo, data string) string {
	t.Helper()
	var out strings.Builder
	if err := r.git.run(context.Background(), strings.NewReader(data), &out, []string{"hash-object", "--stdin"}); err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(out.String())
}

func TestCommitRejectsSymlink(t *testing.T) {
	t.Parallel()
	r := newRepo(t)
	writeVault(t, r)
	if err := os.Remove(filepath.Join(r.Dir, "vault.json")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("identity.pw.age", filepath.Join(r.Dir, "vault.json")); err != nil {
		t.Skip(err)
	}
	if _, err := r.Commit(context.Background(), "bad"); !errors.Is(err, ErrNotAllowed) {
		t.Fatalf("want ErrNotAllowed, got %v", err)
	}
}
