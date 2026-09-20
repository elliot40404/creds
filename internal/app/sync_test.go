package app

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"testing/synctest"
	"time"

	"github.com/elliot40404/creds/internal/gitsync"
)

func withRemote(t *testing.T, s *Service) {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "remote.git")
	if _, err := gitsync.NewGit(filepath.Dir(dir)).Run(context.Background(), "init", "-q", "--bare", "-b", gitsync.Branch, dir); err != nil {
		t.Fatal(err)
	}
	if err := s.RemoteAdd(dir); err != nil {
		t.Fatal(err)
	}
}

func TestSyncWaitsForHeldLock(t *testing.T) {
	t.Parallel()
	s, _, c, _ := initVault(t)
	withRemote(t, s)
	lock, err := gitsync.AcquireLock(s.Paths.SyncLock(), c.t)
	if err != nil {
		t.Fatal(err)
	}
	waits := 0
	s.Waiting = func() {
		waits++
		if err := lock.Release(); err != nil {
			t.Error(err)
		}
	}
	if res, err := s.Sync(); err != nil || res != "pushed" {
		t.Fatalf("sync = %q %v", res, err)
	}
	if waits != 1 {
		t.Fatalf("waits = %d", waits)
	}
	if gitsync.Locked(s.Paths.SyncLock(), c.t) {
		t.Fatal("lock not released")
	}
}

func TestSyncQuietSkipsHeldLock(t *testing.T) {
	t.Parallel()
	s, _, c, _ := initVault(t)
	withRemote(t, s)
	lock, err := gitsync.AcquireLock(s.Paths.SyncLock(), c.t)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SyncQuiet(); err != nil {
		t.Fatal(err)
	}
	st, err := gitsync.LoadState(s.Paths.State())
	if err != nil || !st.LastSync.IsZero() {
		t.Fatalf("synced under held lock: %+v %v", st, err)
	}
	if !gitsync.Locked(s.Paths.SyncLock(), c.t) {
		t.Fatal("held lock removed")
	}
	if err := lock.Release(); err != nil {
		t.Fatal(err)
	}
}

func TestSyncQuietTakesStaleLock(t *testing.T) {
	t.Parallel()
	s, fp, c, _ := initVault(t)
	withRemote(t, s)
	if _, err := gitsync.AcquireLock(s.Paths.SyncLock(), c.t); err != nil {
		t.Fatal(err)
	}
	c.t = c.t.Add(gitsync.LockStale)
	fp.prompts = nil
	if err := s.SyncQuiet(); err != nil {
		t.Fatal(err)
	}
	if len(fp.prompts) != 0 {
		t.Fatalf("prompted: %v", fp.prompts)
	}
	st, err := gitsync.LoadState(s.Paths.State())
	if err != nil || st.LastResult != "pushed" {
		t.Fatalf("state = %+v %v", st, err)
	}
	if gitsync.Locked(s.Paths.SyncLock(), c.t) {
		t.Fatal("lock not released")
	}
}

func TestSyncQuietWithoutSessionNeverPrompts(t *testing.T) {
	t.Parallel()
	s, fp, c, _ := initVault(t)
	withRemote(t, s)
	expire(s, c)
	fp.prompts = nil
	if err := s.SyncQuiet(); err != nil {
		t.Fatal(err)
	}
	if len(fp.prompts) != 0 {
		t.Fatalf("prompted: %v", fp.prompts)
	}
}

func TestSyncQuietNoRemote(t *testing.T) {
	t.Parallel()
	s, _, _, _ := initVault(t)
	if err := s.SyncQuiet(); !errors.Is(err, gitsync.ErrNoRemote) {
		t.Fatalf("err = %v", err)
	}
}

func TestSyncQuietDropsExpiredSession(t *testing.T) {
	t.Parallel()
	s, _, c, _ := initVault(t)
	expire(s, c)
	if err := s.SyncQuiet(); !errors.Is(err, gitsync.ErrNoRemote) {
		t.Fatalf("err = %v", err)
	}
	if _, err := os.Stat(s.Paths.Session()); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("session kept: %v", err)
	}
}

func TestLockDropsExpiredSession(t *testing.T) {
	t.Parallel()
	s, _, c, _ := initVault(t)
	expire(s, c)
	if err := s.Lock(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(s.Paths.Session()); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("session kept: %v", err)
	}
}

func TestSyncQuietWaitsForABusyLock(t *testing.T) {
	t.Parallel()
	s, _, c, _ := initVault(t)
	withRemote(t, s)
	s.Config.Sync.After = time.Second
	lock, err := gitsync.AcquireLock(s.Paths.SyncLock(), c.t)
	if err != nil {
		t.Fatal(err)
	}
	released := make(chan error, 1)
	go func() {
		time.Sleep(1500 * time.Millisecond)
		released <- lock.Release()
	}()
	if err := s.SyncQuiet(); err != nil {
		t.Fatal(err)
	}
	if err := <-released; err != nil {
		t.Fatal(err)
	}
	st, err := gitsync.LoadState(s.Paths.State())
	if err != nil {
		t.Fatal(err)
	}
	if st.LastSync.IsZero() {
		t.Fatal("gave up instead of waiting for the lock")
	}
}

func TestSyncQuietDebouncesWithSyncAfter(t *testing.T) {
	t.Parallel()
	s, _, _, _ := initVault(t)
	withRemote(t, s)
	s.Config.Sync.After = 2 * time.Second
	start := time.Now()
	if err := s.SyncQuiet(); err != nil {
		t.Fatal(err)
	}
	if took := time.Since(start); took < s.Config.Sync.After {
		t.Fatalf("returned after %s, want at least sync.after %s", took, s.Config.Sync.After)
	}
}

func TestSyncQuietRunsAfterALongSyncAfter(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s, _, _, _ := initVault(t)
		withRemote(t, s)
		s.Config.Sync.After = 3 * time.Minute
		if err := s.SyncQuiet(); err != nil {
			t.Fatal(err)
		}
		st, err := gitsync.LoadState(s.Paths.State())
		if err != nil || st.LastSync.IsZero() {
			t.Fatalf("never synced: %+v %v", st, err)
		}
	})
}

func TestSetRemoteKeepsTheOldOneOnABadURL(t *testing.T) {
	t.Parallel()
	s, _, _, _ := initVault(t)
	withRemote(t, s)
	old := s.RemoteURL()
	if err := s.SetRemote("-bad"); !errors.Is(err, gitsync.ErrBadRemote) {
		t.Fatalf("err = %v", err)
	}
	if got := s.RemoteURL(); got != old {
		t.Fatalf("remote %q, want %q", got, old)
	}
	next := filepath.Join(t.TempDir(), "next.git")
	if err := s.SetRemote(next); err != nil {
		t.Fatal(err)
	}
	if got := s.RemoteURL(); got != next {
		t.Fatalf("remote %q, want %q", got, next)
	}
}
