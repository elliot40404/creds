package gitsync

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/elliot40404/creds/internal/vault"
	"github.com/elliot40404/creds/internal/vaultfiles"
)

var fixedNow = time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC)

func newSyncer(t *testing.T, p vaultPair, r *Repo) *Syncer {
	t.Helper()
	return &Syncer{
		Repo:      r,
		StatePath: filepath.Join(t.TempDir(), "state.json"),
		Identity:  p.id,
		Now:       func() time.Time { return fixedNow },
	}
}

func mustState(t *testing.T, path string) State {
	t.Helper()
	st, err := LoadState(path)
	if err != nil {
		t.Fatal(err)
	}
	return st
}

func TestLoadStateMissing(t *testing.T) {
	t.Parallel()
	st := mustState(t, filepath.Join(t.TempDir(), "state.json"))
	if !st.LastSync.IsZero() || st.Conflicts != nil {
		t.Fatalf("state %+v", st)
	}
}

func TestStateRoundtripAndStrict(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "state.json")
	want := State{LastSync: fixedNow, LastResult: "pushed", Conflicts: []PendingConflict{{File: "vault.json"}}}
	if err := SaveState(path, want); err != nil {
		t.Fatal(err)
	}
	got := mustState(t, path)
	if !got.LastSync.Equal(fixedNow) || got.LastResult != "pushed" || got.Conflicts[0].File != "vault.json" {
		t.Fatalf("state %+v", got)
	}
	if err := os.WriteFile(path, []byte(`{"bogus":1}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadState(path); err == nil {
		t.Fatal("unknown key accepted")
	}
}

func TestSyncerRecordsConflictAndSuccess(t *testing.T) {
	t.Parallel()
	p := newVaultPair(t)
	e := secretEntry("e", "0")
	p.put(t, p.a, e)
	p.sync(t, p.a, Pushed)
	p.sync(t, p.b, FastForwarded)
	p.edit(t, p.a, setPassword("remote-secret-value"))
	p.sync(t, p.a, Pushed)
	p.edit(t, p.b, setPassword("local-secret-value"))
	s := newSyncer(t, p, p.b)
	if _, err := s.Sync(context.Background()); !errors.Is(err, ErrConflict) {
		t.Fatalf("want conflict, got %v", err)
	}
	st := mustState(t, s.StatePath)
	if st.LastResult != "conflict" || !st.LastPull.Equal(fixedNow) || len(st.Conflicts) != 1 {
		t.Fatalf("state %+v", st)
	}
	if c := st.Conflicts[0]; len(c.IDs) != 1 || c.IDs[0] != e.ID || c.Paths[0] != "e" {
		t.Fatalf("conflict %+v", c)
	}
	raw, err := os.ReadFile(s.StatePath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "secret-value") {
		t.Fatal("state leaks secrets")
	}
	gitOut(t, p.b, "reset", "-q", "--hard", remoteRef)
	if _, err := s.Sync(context.Background()); err != nil {
		t.Fatal(err)
	}
	if st := mustState(t, s.StatePath); st.LastResult != "up-to-date" || st.Conflicts != nil || st.LastError != "" {
		t.Fatalf("state %+v", st)
	}
}

func TestSyncerRecordsError(t *testing.T) {
	t.Parallel()
	r := newRepo(t)
	s := &Syncer{Repo: r, StatePath: filepath.Join(t.TempDir(), "state.json"), Now: func() time.Time { return fixedNow }}
	if _, err := s.Sync(context.Background()); !errors.Is(err, ErrNoRemote) {
		t.Fatalf("want ErrNoRemote, got %v", err)
	}
	st := mustState(t, s.StatePath)
	if st.LastResult != "error" || st.LastError == "" || !st.LastPull.IsZero() {
		t.Fatalf("state %+v", st)
	}
}

func TestSyncerFlagsPasswordChangeWhenPushFails(t *testing.T) {
	t.Parallel()
	p := newVaultPair(t)
	sy := newSyncer(t, p, p.b)
	p.put(t, p.b, vault.Entry{Path: "b/local", Type: vault.TypeNote})
	writeFile(t, p.a, vaultfiles.PasswordFile, ageBlob+"replayed")
	sealManifest(t, p.a, p.id)
	mustCommit(t, p.a, "replay")
	p.sync(t, p.a, Pushed)
	moved := p.bare + ".away"
	p.b.hook = func(stage string) {
		if stage == "push" {
			_ = os.Rename(p.bare, moved)
		}
	}
	if _, err := sy.Sync(context.Background()); err == nil {
		t.Fatal("want push error")
	}
	if mustState(t, sy.StatePath).PasswordChanged.IsZero() {
		t.Fatal("password change not flagged after failed push")
	}
}

func TestSyncerSkipsStateAfterLockLoss(t *testing.T) {
	t.Parallel()
	p := newVaultPair(t)
	s := newSyncer(t, p, p.b)
	want := State{LastResult: "conflict", Conflicts: []PendingConflict{{File: vaultfiles.MetaFile, Choice: vault.Theirs}}}
	if err := SaveState(s.StatePath, want); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancelCause(context.Background())
	cancel(ErrLockLost)
	if _, err := s.Sync(ctx); !errors.Is(err, ErrLockLost) {
		t.Fatalf("want ErrLockLost, got %v", err)
	}
	st := mustState(t, s.StatePath)
	if st.LastResult != "conflict" || len(st.Conflicts) != 1 || st.Conflicts[0].Choice != vault.Theirs {
		t.Fatalf("state rewritten after lock loss: %+v", st)
	}
}

func TestRecordClearsPendingOnSuccessOnly(t *testing.T) {
	at := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	cases := map[string]struct {
		res   SyncResult
		err   error
		clear bool
	}{
		"pushed":     {Pushed, nil, true},
		"up to date": {UpToDate, nil, true},
		"error":      {UpToDate, errors.New("network down"), false},
		"locked out": {NeedsUnlock, nil, false},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			st := State{PendingSince: at}
			st.record(at.Add(time.Minute), tc.res, tc.err)
			if got := st.PendingSince.IsZero(); got != tc.clear {
				t.Fatalf("pending cleared %v, want %v", got, tc.clear)
			}
		})
	}
}

func TestMarkPendingKeepsTheFirstStamp(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	first := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	if _, err := MarkPending(path, first); err != nil {
		t.Fatal(err)
	}
	if _, err := MarkPending(path, first.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	st, err := LoadState(path)
	if err != nil {
		t.Fatal(err)
	}
	if !st.PendingSince.Equal(first) {
		t.Fatalf("pending %v, want %v", st.PendingSince, first)
	}
	if err := MarkSpawn(path, first.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if st, _ = LoadState(path); !st.LastSpawn.Equal(first.Add(time.Minute)) {
		t.Fatalf("spawn %v", st.LastSpawn)
	}
	if !st.PendingSince.Equal(first) {
		t.Fatal("spawn cleared pending")
	}
}

func TestSyncKeepsStateWrittenWhileItRuns(t *testing.T) {
	t.Parallel()
	p := newVaultPair(t)
	sy := newSyncer(t, p, p.b)
	spawned := fixedNow.Add(-time.Minute)
	p.b.hook = func(stage string) {
		if stage == "fetch" {
			if err := MarkSpawn(sy.StatePath, spawned); err != nil {
				t.Error(err)
			}
		}
	}
	if _, err := sy.Sync(context.Background()); err != nil {
		t.Fatal(err)
	}
	if st := mustState(t, sy.StatePath); !st.LastSpawn.Equal(spawned) || st.LastSync.IsZero() {
		t.Fatalf("state %+v", st)
	}
}
