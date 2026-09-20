package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/elliot40404/creds/internal/format"
	"github.com/elliot40404/creds/internal/gitsync"
	"github.com/elliot40404/creds/internal/manifest"
	"github.com/elliot40404/creds/internal/vault"
	"github.com/elliot40404/creds/internal/vaultfiles"
)

const oldIgnoreRules = "*\n!.gitignore\n!vault.json\n!identity.pw.age\n!identity.recovery.age\n!entries/\n!entries/*.enc\n"

func downgradeToV2(t *testing.T, s *Service) {
	t.Helper()
	meta, err := format.LoadMeta(s.vaultFile(vaultfiles.MetaFile))
	if err != nil {
		t.Fatal(err)
	}
	v2 := fmt.Sprintf(`{"format_version":2,"recipient":%q,"created":%q}`, meta.Recipient, meta.Created.Format(time.RFC3339))
	if err := os.WriteFile(s.vaultFile(vaultfiles.MetaFile), []byte(v2), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(s.vaultFile(vaultfiles.IgnoreFile), []byte(oldIgnoreRules), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(s.vaultFile(vaultfiles.ManifestFile)); err != nil {
		t.Fatal(err)
	}
	g := gitsync.NewGit(s.Paths.Vault())
	for _, args := range [][]string{
		{"add", "-A"},
		{"-c", "user.name=x", "-c", "user.email=x@x", "commit", "-q", "-m", "v2"},
	} {
		if _, err := g.Run(context.Background(), args...); err != nil {
			t.Fatal(err)
		}
	}
}

func seedEntries(t *testing.T, s *Service) []vault.Entry {
	t.Helper()
	var out []vault.Entry
	for _, p := range []string{"prod/db", "web/mail", "ssh/box"} {
		e, err := s.Add(dbEntry(p))
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, e)
	}
	return out
}

func TestUnlockUpgradesV2AndKeepsEverySecret(t *testing.T) {
	t.Parallel()
	s, fp, _, _ := initVault(t)
	want := seedEntries(t, s)
	downgradeToV2(t, s)

	if err := unlockWith(t, s, fp, mainWord); err != nil {
		t.Fatal(err)
	}
	meta, err := format.LoadMeta(s.vaultFile(vaultfiles.MetaFile))
	if err != nil || meta.FormatVersion != format.CurrentVersion {
		t.Fatalf("meta = %+v %v", meta, err)
	}
	for _, w := range want {
		got, err := s.Get(w.Path)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got, w) {
			t.Fatalf("%s changed:\n got %+v\nwant %+v", w.Path, got, w)
		}
	}
}

func TestUpgradeWritesTheManifestAndOpensTheIgnoreFile(t *testing.T) {
	t.Parallel()
	s, fp, _, _ := initVault(t)
	seedEntries(t, s)
	downgradeToV2(t, s)

	if err := unlockWith(t, s, fp, mainWord); err != nil {
		t.Fatal(err)
	}
	id, err := s.identity()
	if err != nil {
		t.Fatal(err)
	}
	m, err := manifest.Verify(s.Paths.Vault(), id)
	if err != nil {
		t.Fatal(err)
	}
	if m.Generation != 1 || len(m.Parents) != 0 {
		t.Fatalf("manifest %+v", m)
	}
	got, err := os.ReadFile(s.vaultFile(vaultfiles.IgnoreFile))
	if err != nil || string(got) != vaultfiles.IgnoreRules {
		t.Fatalf("ignore file %q %v", got, err)
	}
}

func TestFailedUpgradeLeavesTheV2VaultUsable(t *testing.T) {
	t.Parallel()
	s, fp, _, _ := initVault(t)
	want := seedEntries(t, s)
	downgradeToV2(t, s)
	if err := os.Mkdir(s.vaultFile(".migrate-backup"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(s.vaultFile(".migrate-backup/evil.txt"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := unlockWith(t, s, fp, mainWord); err == nil {
		t.Fatal("upgrade ran over a dirty backup dir")
	}
	if err := os.RemoveAll(s.vaultFile(".migrate-backup")); err != nil {
		t.Fatal(err)
	}
	meta, err := format.LoadMeta(s.vaultFile(vaultfiles.MetaFile))
	if !errors.Is(err, format.ErrOldVersion) {
		t.Fatalf("meta = %+v %v", meta, err)
	}
	if err := unlockWith(t, s, fp, mainWord); err != nil {
		t.Fatal(err)
	}
	for _, w := range want {
		if _, err := s.Get(w.Path); err != nil {
			t.Fatalf("%s: %v", w.Path, err)
		}
	}
}

func TestUpgradeWaitsForTheSyncLock(t *testing.T) {
	t.Parallel()
	s, fp, c, _ := initVault(t)
	seedEntries(t, s)
	downgradeToV2(t, s)
	lock, err := gitsync.AcquireLock(s.Paths.SyncLock(), c.t)
	if err != nil {
		t.Fatal(err)
	}
	waits := 0
	s.Waiting = func() {
		waits++
		if _, err := format.LoadMeta(s.vaultFile(vaultfiles.MetaFile)); !errors.Is(err, format.ErrOldVersion) {
			t.Errorf("upgraded while locked: %v", err)
		}
		if err := lock.Release(); err != nil {
			t.Error(err)
		}
	}
	if err := unlockWith(t, s, fp, mainWord); err != nil {
		t.Fatal(err)
	}
	if waits != 1 {
		t.Fatalf("waits = %d", waits)
	}
	meta, err := format.LoadMeta(s.vaultFile(vaultfiles.MetaFile))
	if err != nil || meta.FormatVersion != format.CurrentVersion {
		t.Fatalf("meta = %+v %v", meta, err)
	}
}
