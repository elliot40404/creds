package gitsync

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/elliot40404/creds/internal/crypto"

	"github.com/elliot40404/creds/internal/vault"
	"github.com/elliot40404/creds/internal/vaultfiles"
)

func headOf(t *testing.T, r *Repo) string {
	t.Helper()
	return gitOut(t, r, "rev-parse", "HEAD")
}

func assertSyncRefused(t *testing.T, p vaultPair, r *Repo, target error) {
	t.Helper()
	head := headOf(t, r)
	if _, err := r.Sync(context.Background(), p.opts()); !errors.Is(err, target) {
		t.Fatalf("want %v, got %v", target, err)
	}
	if got := headOf(t, r); got != head {
		t.Fatalf("head moved %s -> %s", head, got)
	}
	for _, f := range vaultfiles.Required() {
		if _, err := os.Lstat(filepath.Join(r.Dir, filepath.FromSlash(f))); err != nil {
			t.Fatalf("%s: %v", f, err)
		}
	}
}

func TestSyncRefusesRemoteDeletion(t *testing.T) {
	t.Parallel()
	targets := append(vaultfiles.Fixed(), vaultfiles.EntriesDir+"/03.enc")
	for _, f := range targets {
		t.Run(f, func(t *testing.T) {
			t.Parallel()
			p := newVaultPair(t)
			if err := os.Remove(filepath.Join(p.a.Dir, filepath.FromSlash(f))); err != nil {
				t.Fatal(err)
			}
			mustCommit(t, p.a, "delete")
			p.sync(t, p.a, Pushed)
			assertSyncRefused(t, p, p.b, ErrIncomplete)
		})
	}
}

func TestMergeRefusesRemoteDeletion(t *testing.T) {
	t.Parallel()
	p := newVaultPair(t)
	p.put(t, p.b, vault.Entry{Path: "b/local", Type: vault.TypeNote})
	if err := os.Remove(filepath.Join(p.a.Dir, vaultfiles.MetaFile)); err != nil {
		t.Fatal(err)
	}
	mustCommit(t, p.a, "delete")
	p.sync(t, p.a, Pushed)
	assertSyncRefused(t, p, p.b, ErrIncomplete)
}

func TestCloneRefusesIncompleteVault(t *testing.T) {
	t.Parallel()
	p := newVaultPair(t)
	if err := os.Remove(filepath.Join(p.a.Dir, vaultfiles.EntriesDir, "03.enc")); err != nil {
		t.Fatal(err)
	}
	mustCommit(t, p.a, "delete")
	p.sync(t, p.a, Pushed)
	dir := filepath.Join(t.TempDir(), "vault")
	if _, err := Clone(context.Background(), p.bare, dir); !errors.Is(err, ErrIncomplete) {
		t.Fatalf("want ErrIncomplete, got %v", err)
	}
}

func TestSyncRefusesRecoverySwap(t *testing.T) {
	t.Parallel()
	p := newVaultPair(t)
	writeFile(t, p.a, vaultfiles.RecoveryFile, ageBlob+"garbage")
	mustCommit(t, p.a, "swap")
	p.sync(t, p.a, Pushed)
	assertSyncRefused(t, p, p.b, ErrProtected)
}

func TestMergeRefusesRecoverySwap(t *testing.T) {
	t.Parallel()
	p := newVaultPair(t)
	p.put(t, p.b, vault.Entry{Path: "b/local", Type: vault.TypeNote})
	writeFile(t, p.a, vaultfiles.RecoveryFile, ageBlob+"garbage")
	mustCommit(t, p.a, "swap")
	p.sync(t, p.a, Pushed)
	assertSyncRefused(t, p, p.b, ErrProtected)
}

func TestSyncRefusesRecipientChange(t *testing.T) {
	t.Parallel()
	p := newVaultPair(t)
	writeFile(t, p.a, vaultfiles.MetaFile, `{"format_version":3,"recipient":"age1evil"}`)
	mustCommit(t, p.a, "recipient")
	p.sync(t, p.a, Pushed)
	assertSyncRefused(t, p, p.b, ErrProtected)
}

func TestSyncAcceptsMetaChangeWithSameRecipient(t *testing.T) {
	t.Parallel()
	p := newVaultPair(t)
	meta := `{"format_version":3,"recipient":"` + p.id.Recipient().String() + `","created":"2026-01-01T00:00:00Z"}`
	writeFile(t, p.a, vaultfiles.MetaFile, meta)
	sealManifest(t, p.a, p.id)
	mustCommit(t, p.a, "meta")
	p.sync(t, p.a, Pushed)
	p.sync(t, p.b, FastForwarded)
}

func forgeBuckets(t *testing.T, p vaultPair, r *Repo) {
	t.Helper()
	for slot := range vaultfiles.BucketCount {
		data := `{"mac":"00","bucket":{"format":1,"slot":` + strconv.Itoa(slot) + `,"entries":[]}}`
		forged, err := crypto.Seal(p.id.Recipient(), []byte(data))
		if err != nil {
			t.Fatal(err)
		}
		writeFile(t, r, vaultfiles.EntriesDir+"/"+vaultfiles.BucketName(slot), string(forged))
	}
	mustCommit(t, r, "forged")
	p.sync(t, r, Pushed)
}

func TestFastForwardRefusesForgedBuckets(t *testing.T) {
	t.Parallel()
	p := newVaultPair(t)
	p.put(t, p.b, secretEntry("keep", "1"))
	p.sync(t, p.b, Pushed)
	p.sync(t, p.a, FastForwarded)
	forgeBuckets(t, p, p.a)
	assertSyncRefused(t, p, p.b, vault.ErrBadBucket)
	if got := p.password(t, p.b, "keep"); got != "1" {
		t.Fatalf("password %q", got)
	}
}

func TestFastForwardWithoutIdentityWaitsForUnlock(t *testing.T) {
	t.Parallel()
	p := newVaultPair(t)
	p.put(t, p.a, secretEntry("new", "1"))
	p.sync(t, p.a, Pushed)
	head := headOf(t, p.b)
	got, err := p.b.Sync(context.Background(), SyncOptions{})
	if err != nil || got != NeedsUnlock {
		t.Fatalf("got %v, %v", got, err)
	}
	if headOf(t, p.b) != head {
		t.Fatal("head moved without identity")
	}
	p.sync(t, p.b, FastForwarded)
	if got := p.password(t, p.b, "new"); got != "1" {
		t.Fatalf("password %q", got)
	}
}

func metaWithVersion(p vaultPair, v int) string {
	return `{"format_version":` + strconv.Itoa(v) + `,"recipient":"` + p.id.Recipient().String() + `","created":"2026-01-01T00:00:00Z"}`
}

func TestSyncRefusesNewerFormatVersion(t *testing.T) {
	t.Parallel()
	p := newVaultPair(t)
	writeFile(t, p.a, vaultfiles.MetaFile, metaWithVersion(p, 99))
	mustCommit(t, p.a, "future")
	p.sync(t, p.a, Pushed)
	assertSyncRefused(t, p, p.b, ErrBadVersion)
}

func TestSyncRefusesOlderFormatVersion(t *testing.T) {
	t.Parallel()
	p := newVaultPair(t)
	writeFile(t, p.a, vaultfiles.MetaFile, metaWithVersion(p, 0))
	mustCommit(t, p.a, "old")
	p.sync(t, p.a, Pushed)
	assertSyncRefused(t, p, p.b, ErrBadVersion)
}

func TestCloneRefusesNewerFormatVersion(t *testing.T) {
	t.Parallel()
	p := newVaultPair(t)
	writeFile(t, p.a, vaultfiles.MetaFile, metaWithVersion(p, 99))
	mustCommit(t, p.a, "future")
	p.sync(t, p.a, Pushed)
	dir := filepath.Join(t.TempDir(), "vault")
	if _, err := Clone(context.Background(), p.bare, dir); !errors.Is(err, ErrBadVersion) {
		t.Fatalf("want ErrBadVersion, got %v", err)
	}
}

func TestSyncRefusesMetaTheLoaderRejects(t *testing.T) {
	t.Parallel()
	cases := map[string]func(p vaultPair) string{
		"unknown member": func(p vaultPair) string {
			return `{"format_version":3,"recipient":"` + p.id.Recipient().String() + `","created":"2026-01-01T00:00:00Z","x":1}`
		},
		"bad created": func(p vaultPair) string {
			return `{"format_version":3,"recipient":"` + p.id.Recipient().String() + `","created":"soon"}`
		},
		"too large": func(p vaultPair) string {
			return metaWithVersion(p, 1) + strings.Repeat(" ", 2<<20)
		},
	}
	for name, meta := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			p := newVaultPair(t)
			p.put(t, p.b, secretEntry("keep", "1"))
			p.sync(t, p.b, Pushed)
			p.sync(t, p.a, FastForwarded)
			writeFile(t, p.a, vaultfiles.MetaFile, meta(p))
			mustCommit(t, p.a, "meta")
			p.sync(t, p.a, Pushed)
			assertSyncRefused(t, p, p.b, ErrBadMeta)
			if got := p.password(t, p.b, "keep"); got != "1" {
				t.Fatalf("password %q", got)
			}
		})
	}
}

func TestFastForwardRefusesReplayedDuplicatePath(t *testing.T) {
	t.Parallel()
	p := newVaultPair(t)
	old := secretEntry("work/db", "1")
	old.ID = idNearSlot(0, true)
	p.put(t, p.a, old)
	p.sync(t, p.a, Pushed)
	oldRev := headOf(t, p.a)
	fresh := secretEntry("work/db", "2")
	fresh.ID = idNearSlot(0, false)
	p.edit(t, p.a, func(v *vault.Vault) error {
		if err := v.Delete("work/db"); err != nil {
			return err
		}
		_, err := v.Put(fresh)
		return err
	})
	p.sync(t, p.a, Pushed)
	p.sync(t, p.b, FastForwarded)
	gitOut(t, p.a, "checkout", oldRev, "--", vaultfiles.EntriesDir+"/"+vaultfiles.BucketName(0))
	mustCommit(t, p.a, "replay")
	p.sync(t, p.a, Pushed)
	assertSyncRefused(t, p, p.b, vault.ErrBadBucket)
	if got := p.password(t, p.b, "work/db"); got != "2" {
		t.Fatalf("password %q", got)
	}
}
