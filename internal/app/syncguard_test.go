package app

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/elliot40404/creds/internal/crypto"
	"github.com/elliot40404/creds/internal/gitsync"
	"github.com/elliot40404/creds/internal/testutil"
	"github.com/elliot40404/creds/internal/vault"
	"github.com/elliot40404/creds/internal/vaultfiles"
)

func pushHostile(t *testing.T, s *Service, change func(dir string)) {
	t.Helper()
	ctx := context.Background()
	r, err := s.repo(ctx)
	if err != nil {
		t.Fatal(err)
	}
	url, err := r.RemoteURL(ctx)
	if err != nil {
		t.Fatal(err)
	}
	testutil.PushChange(t, url, gitsync.Branch, "hostile", change)
}

func writeHostile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func syncedVault(t *testing.T) *Service {
	t.Helper()
	s, _, _, _ := initVault(t)
	if _, err := s.Add(dbEntry("work/db")); err != nil {
		t.Fatal(err)
	}
	withRemote(t, s)
	if res, err := s.Sync(); err != nil || res != "pushed" {
		t.Fatalf("sync = %q %v", res, err)
	}
	return s
}

func assertSyncBlocked(t *testing.T, s *Service, target error) {
	t.Helper()
	if _, err := s.Sync(); !errors.Is(err, target) {
		t.Fatalf("want %v, got %v", target, err)
	}
	if err := s.SyncQuiet(); !errors.Is(err, target) {
		t.Fatalf("quiet: want %v, got %v", target, err)
	}
	list, err := s.List()
	if err != nil || len(list) != 1 {
		t.Fatalf("list = %v %v", list, err)
	}
}

func TestSyncRefusesForgedBuckets(t *testing.T) {
	t.Parallel()
	s := syncedVault(t)
	id, err := s.identity()
	if err != nil {
		t.Fatal(err)
	}
	pushHostile(t, s, func(dir string) {
		for slot := range vaultfiles.BucketCount {
			data := `{"mac":"00","bucket":{"format":1,"slot":` + strconv.Itoa(slot) + `,"entries":[]}}`
			forged, err := crypto.Seal(id.Recipient(), []byte(data))
			if err != nil {
				t.Fatal(err)
			}
			writeHostile(t, filepath.Join(dir, vaultfiles.EntriesDir, vaultfiles.BucketName(slot)), forged)
		}
	})
	assertSyncBlocked(t, s, vault.ErrBadBucket)
}

func TestSyncRefusesFutureFormatVersion(t *testing.T) {
	t.Parallel()
	s := syncedVault(t)
	id, err := s.identity()
	if err != nil {
		t.Fatal(err)
	}
	pushHostile(t, s, func(dir string) {
		meta := `{"format_version":99,"recipient":"` + id.Recipient().String() + `","created":"2026-01-01T00:00:00Z"}`
		writeHostile(t, filepath.Join(dir, vaultfiles.MetaFile), []byte(meta))
	})
	assertSyncBlocked(t, s, gitsync.ErrBadVersion)
}

func TestSyncQuietWithoutSessionKeepsChangedFiles(t *testing.T) {
	t.Parallel()
	s := syncedVault(t)
	before, err := os.ReadFile(s.vaultFile(vaultfiles.PasswordFile))
	if err != nil {
		t.Fatal(err)
	}
	id, err := s.identity()
	if err != nil {
		t.Fatal(err)
	}
	swapped, err := crypto.Seal(id.Recipient(), []byte("older wrapper"))
	if err != nil {
		t.Fatal(err)
	}
	pushHostile(t, s, func(dir string) {
		writeHostile(t, filepath.Join(dir, vaultfiles.PasswordFile), swapped)
	})
	if err := s.sessions().Delete(); err != nil {
		t.Fatal(err)
	}
	if err := s.SyncQuiet(); err != nil {
		t.Fatal(err)
	}
	st, err := gitsync.LoadState(s.Paths.State())
	if err != nil || st.LastResult != gitsync.NeedsUnlock.String() {
		t.Fatalf("result %q %v", st.LastResult, err)
	}
	after, err := os.ReadFile(s.vaultFile(vaultfiles.PasswordFile))
	if err != nil || !bytes.Equal(after, before) {
		t.Fatalf("password file changed: %v", err)
	}
}
