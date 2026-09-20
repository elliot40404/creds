package gitsync

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/elliot40404/creds/internal/anchor"
	"github.com/elliot40404/creds/internal/vault"
	"github.com/elliot40404/creds/internal/vaultfiles"
)

func anchoredOpts(t *testing.T, p vaultPair) SyncOptions {
	t.Helper()
	opts := p.opts()
	opts.AnchorPath = filepath.Join(t.TempDir(), "anchor.json")
	return opts
}

func syncWith(t *testing.T, r *Repo, opts SyncOptions, want SyncResult) {
	t.Helper()
	got, err := r.Sync(context.Background(), opts)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("sync %v, want %v", got, want)
	}
}

func TestSyncRecordsAnchorAndRefusesRollback(t *testing.T) {
	t.Parallel()
	p := newVaultPair(t)
	opts := anchoredOpts(t, p)
	old := bareHead(t, p.bare)
	p.put(t, p.a, vault.Entry{Path: "a/one", Type: vault.TypeNote})
	p.sync(t, p.a, Pushed)
	syncWith(t, p.b, opts, FastForwarded)

	a, err := anchor.Load(opts.AnchorPath)
	if err != nil {
		t.Fatal(err)
	}
	if a.Generation < 2 || a.Manifest == "" {
		t.Fatalf("anchor %+v", a)
	}

	setBareHead(t, p.bare, old)
	fresh := cloneRepo(t, p.bare)
	if _, err := fresh.Sync(context.Background(), opts); !errors.Is(err, anchor.ErrRollback) {
		t.Fatalf("want rollback, got %v", err)
	}
	after, err := anchor.Load(opts.AnchorPath)
	if err != nil || after.Generation != a.Generation || after.Manifest != a.Manifest {
		t.Fatalf("anchor changed on refusal: %+v %v", after, err)
	}
}

func TestMergeRefusesOlderGenuineTree(t *testing.T) {
	t.Parallel()
	p := newVaultPair(t)
	opts := anchoredOpts(t, p)
	p.put(t, p.a, vault.Entry{Path: "a/one", Type: vault.TypeNote})
	p.sync(t, p.a, Pushed)
	older := bareHead(t, p.bare)
	p.put(t, p.a, vault.Entry{Path: "a/two", Type: vault.TypeNote})
	p.sync(t, p.a, Pushed)
	base := bareHead(t, p.bare)
	syncWith(t, p.b, opts, FastForwarded)
	trusted, err := anchor.Load(opts.AnchorPath)
	if err != nil {
		t.Fatal(err)
	}

	p.put(t, p.b, vault.Entry{Path: "b/three", Type: vault.TypeNote})
	g := Open(p.bare)
	tr := gitOut(t, g, "rev-parse", older+"^{tree}")
	setBareHead(t, p.bare, gitOut(t, g, "commit-tree", tr, "-p", base, "-m", "replay"))

	head := headOf(t, p.b)
	if _, err := p.b.Sync(context.Background(), opts); !errors.Is(err, anchor.ErrRollback) {
		t.Fatalf("want rollback, got %v", err)
	}
	if got := headOf(t, p.b); got != head {
		t.Fatalf("head moved %s -> %s", head, got)
	}
	after, err := anchor.Load(opts.AnchorPath)
	if err != nil || after != trusted {
		t.Fatalf("anchor changed on refusal: %+v %v", after, err)
	}
}

func TestMergeAcceptsHonestRemoteBelowAnchor(t *testing.T) {
	t.Parallel()
	p := newVaultPair(t)
	opts := anchoredOpts(t, p)
	syncWith(t, p.b, opts, UpToDate)
	p.put(t, p.b, vault.Entry{Path: "b/one", Type: vault.TypeNote})
	p.put(t, p.b, vault.Entry{Path: "b/two", Type: vault.TypeNote})
	if err := p.b.advanceAnchor(context.Background(), opts, Pushed); err != nil {
		t.Fatal(err)
	}
	p.put(t, p.a, vault.Entry{Path: "a/one", Type: vault.TypeNote})
	p.sync(t, p.a, Pushed)
	syncWith(t, p.b, opts, Merged)
}

func TestFastForwardRefusesReplayedBuckets(t *testing.T) {
	t.Parallel()
	p := newVaultPair(t)
	p.put(t, p.a, vault.Entry{Path: "a/one", Type: vault.TypeNote})
	p.sync(t, p.a, Pushed)
	p.sync(t, p.b, FastForwarded)

	stale := snapshotBuckets(t, p.a)
	p.put(t, p.a, vault.Entry{Path: "a/two", Type: vault.TypeNote})
	p.sync(t, p.a, Pushed)
	restoreBuckets(t, p.a, stale)
	if !mustCommit(t, p.a, "replay") {
		t.Fatal("nothing committed")
	}
	if _, err := p.a.push(context.Background()); err != nil {
		t.Fatal(err)
	}

	assertSyncRefused(t, p, p.b, ErrUnverified)
}

func TestMergeWritesTwoParentManifest(t *testing.T) {
	t.Parallel()
	p := newVaultPair(t)
	p.put(t, p.a, vault.Entry{Path: "a/mine", Type: vault.TypeNote})
	p.sync(t, p.a, Pushed)
	p.put(t, p.b, vault.Entry{Path: "b/yours", Type: vault.TypeNote})
	p.sync(t, p.b, Merged)

	ctx := context.Background()
	head, err := p.b.Head(ctx)
	if err != nil {
		t.Fatal(err)
	}
	tr, err := p.b.readTree(ctx, head)
	if err != nil {
		t.Fatal(err)
	}
	m, err := p.b.treeManifest(ctx, tr, p.id)
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Parents) != 2 {
		t.Fatalf("parents %v", m.Parents)
	}
	if m.Generation < 3 {
		t.Fatalf("generation %d", m.Generation)
	}
}

func snapshotBuckets(t *testing.T, r *Repo) map[string][]byte {
	t.Helper()
	out := map[string][]byte{}
	for slot := range vaultfiles.BucketCount {
		name := vaultfiles.EntriesDir + "/" + vaultfiles.BucketName(slot)
		data, err := os.ReadFile(filepath.Join(r.Dir, filepath.FromSlash(name)))
		if err != nil {
			t.Fatal(err)
		}
		out[name] = data
	}
	return out
}

func restoreBuckets(t *testing.T, r *Repo, snap map[string][]byte) {
	t.Helper()
	for name, data := range snap {
		if err := os.WriteFile(filepath.Join(r.Dir, filepath.FromSlash(name)), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
}
