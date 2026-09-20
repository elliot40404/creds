package gitsync

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func pushHostile(t *testing.T, r *Repo, records ...string) {
	t.Helper()
	blob := mustBlob(t, r, "evil")
	var in strings.Builder
	for _, rec := range records {
		mode, name, _ := strings.Cut(rec, " ")
		kind, sha := "blob", blob
		if mode == "160000" {
			kind, sha = "commit", gitOut(t, r, "rev-parse", "HEAD")
		}
		in.WriteString(mode + " " + kind + " " + sha + "\t" + name + "\x00")
	}
	parent := gitOut(t, r, "rev-parse", "HEAD")
	base, err := r.readTree(context.Background(), parent)
	if err != nil {
		t.Fatal(err)
	}
	for name, sha := range base {
		if !strings.Contains(name, "/") {
			in.WriteString(blobMode + " blob " + sha + "\t" + name + "\x00")
		}
	}
	tree, err := r.pipe(context.Background(), []byte(in.String()), "mktree", "-z")
	if err != nil {
		t.Fatal(err)
	}
	commit := gitOut(t, r, "commit-tree", tree, "-p", parent, "-m", "evil")
	gitOut(t, r, "push", "-q", Remote, commit+":"+localRef)
}

func mustBlob(t *testing.T, r *Repo, data string) string {
	t.Helper()
	sha, err := r.writeBlob(context.Background(), []byte(data))
	if err != nil {
		t.Fatal(err)
	}
	return sha
}

var hostileTrees = map[string][]string{
	"gitattributes": {"100644 .gitattributes"},
	"symlink":       {"120000 entries"},
	"gitlink":       {"160000 mod"},
	"script":        {"100755 evil.sh"},
	"backup":        {"100644 .migrate-backup"},
}

func TestFastForwardRejectsHostileTree(t *testing.T) {
	t.Parallel()
	for name, recs := range hostileTrees {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			p := newPair(t)
			head := gitOut(t, p.b, "rev-parse", "HEAD")
			pushHostile(t, p.a, recs...)
			if _, err := p.b.Sync(context.Background(), SyncOptions{}); !errors.Is(err, ErrNotAllowed) {
				t.Fatalf("want ErrNotAllowed, got %v", err)
			}
			if gitOut(t, p.b, "rev-parse", "HEAD") != head {
				t.Fatal("head moved")
			}
			assertAbsent(t, p.b.Dir, recs)
		})
	}
}

func TestFirstSyncRejectsHostileTree(t *testing.T) {
	t.Parallel()
	p := newPair(t)
	pushHostile(t, p.a, "120000 entries")
	r := newRepo(t)
	if err := r.RemoteAdd(context.Background(), p.bare); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Sync(context.Background(), SyncOptions{}); !errors.Is(err, ErrNotAllowed) {
		t.Fatalf("want ErrNotAllowed, got %v", err)
	}
	if got, err := r.rev(context.Background(), localRef); err != nil || got != "" {
		t.Fatalf("head set to %q %v", got, err)
	}
}

func assertAbsent(t *testing.T, dir string, recs []string) {
	t.Helper()
	for _, rec := range recs {
		_, name, _ := strings.Cut(rec, " ")
		if name == "entries" {
			info, err := os.Lstat(filepath.Join(dir, name))
			if err == nil && info.Mode()&fs.ModeSymlink != 0 {
				t.Fatal("entries replaced by symlink")
			}
			continue
		}
		if _, err := os.Lstat(filepath.Join(dir, name)); !errors.Is(err, fs.ErrNotExist) {
			t.Fatalf("%s present: %v", name, err)
		}
	}
}
