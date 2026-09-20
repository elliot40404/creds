package gitsync

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/elliot40404/creds/internal/vaultfiles"
)

func TestSyncFlagsRemotePasswordChange(t *testing.T) {
	t.Parallel()
	p := newVaultPair(t)
	sy := newSyncer(t, p, p.b)
	writeFile(t, p.b, vaultfiles.PasswordFile, ageBlob+"local")
	sealManifest(t, p.b, p.id)
	mustCommit(t, p.b, "passwd")
	if res, err := sy.Sync(context.Background()); err != nil || res != Pushed {
		t.Fatalf("push %v %v", res, err)
	}
	if st := mustState(t, sy.StatePath); !st.PasswordChanged.IsZero() {
		t.Fatal("local passwd flagged")
	}
	p.sync(t, p.a, FastForwarded)
	writeFile(t, p.a, vaultfiles.PasswordFile, ageBlob+"replayed")
	sealManifest(t, p.a, p.id)
	mustCommit(t, p.a, "replay")
	p.sync(t, p.a, Pushed)
	if res, err := sy.Sync(context.Background()); err != nil || res != FastForwarded {
		t.Fatalf("pull %v %v", res, err)
	}
	if st := mustState(t, sy.StatePath); !st.PasswordChanged.Equal(fixedNow) {
		t.Fatalf("change not flagged: %+v", st)
	}
	lock := filepath.Join(t.TempDir(), "sync.lock")
	held, err := AcquireLock(lock, fixedNow)
	if err != nil {
		t.Fatal(err)
	}
	if err := AckPasswordChange(sy.StatePath, lock, fixedNow); !errors.Is(err, ErrLocked) {
		t.Fatalf("want ErrLocked, got %v", err)
	}
	if err := held.Release(); err != nil {
		t.Fatal(err)
	}
	if err := AckPasswordChange(sy.StatePath, lock, fixedNow); err != nil {
		t.Fatal(err)
	}
	if st := mustState(t, sy.StatePath); !st.PasswordChanged.IsZero() || st.LastResult != "fast-forwarded" {
		t.Fatalf("ack state %+v", st)
	}
}
