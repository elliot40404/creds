package gitsync

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func holdLock(t *testing.T, path string, now time.Time) (*Lock, chan time.Time) {
	t.Helper()
	tick := make(chan time.Time)
	l, err := acquire(path, now, tick)
	if err != nil {
		t.Fatal(err)
	}
	return l, tick
}

func beat(tick chan<- time.Time, at time.Time) {
	tick <- at
	tick <- at
}

func TestLiveHolderKeepsLockPastStale(t *testing.T) {
	t.Parallel()
	path := lockPath(t)
	start := time.Now()
	l, tick := holdLock(t, path, start)
	at := start
	for !at.After(start.Add(LockStale)) {
		at = at.Add(lockRefresh)
		beat(tick, at)
	}
	if info, err := readInfo(path); err != nil || !info.Time.Equal(at.UTC()) {
		t.Fatalf("lock not refreshed %+v %v", info, err)
	}
	if !Locked(path, at) {
		t.Fatal("refreshed lock reported stale")
	}
	if _, err := AcquireLock(path, at); !errors.Is(err, ErrLocked) {
		t.Fatalf("lock stolen from live holder: %v", err)
	}
	if err := l.Release(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("lock file left: %v", err)
	}
}

func TestRefreshKeepsPermsAndOwner(t *testing.T) {
	t.Parallel()
	path := lockPath(t)
	now := time.Now()
	l, err := AcquireLock(path, now)
	if err != nil {
		t.Fatal(err)
	}
	before, err := readInfo(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := l.refresh(now.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	after, err := readInfo(path)
	if err != nil {
		t.Fatal(err)
	}
	if !after.Time.After(before.Time) || after.PID != before.PID {
		t.Fatalf("refresh wrote %+v, was %+v", after, before)
	}
	st, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if perm := st.Mode().Perm(); perm != 0o600 && perm != 0o666 {
		t.Fatalf("lock perms %v", perm)
	}
	if entries, err := os.ReadDir(filepath.Dir(path)); err != nil || len(entries) != 2 {
		t.Fatalf("dir entries %v %v", entries, err)
	}
	if err := l.Release(); err != nil {
		t.Fatal(err)
	}
}

func TestRefreshOnStolenLockReportsLost(t *testing.T) {
	t.Parallel()
	path := lockPath(t)
	now := time.Now()
	l, err := AcquireLock(path, now)
	if err != nil {
		t.Fatal(err)
	}
	thief := &Lock{path: path, info: lockInfo{PID: os.Getpid(), Time: now.Add(time.Hour).UTC()}}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := thief.create(); err != nil {
		t.Fatal(err)
	}
	if err := l.refresh(time.Now()); !errors.Is(err, ErrLockLost) {
		t.Fatalf("refresh = %v", err)
	}
	if err := l.Release(); err != nil {
		t.Fatal(err)
	}
	if !Locked(path, now.Add(time.Hour)) {
		t.Fatal("old holder released the thief lock")
	}
	if err := thief.Release(); err != nil {
		t.Fatal(err)
	}
}

func TestKeepaliveStopsAfterLoss(t *testing.T) {
	t.Parallel()
	path := lockPath(t)
	now := time.Now()
	l, tick := holdLock(t, path, now)
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	tick <- now.Add(lockRefresh)
	if err := l.Release(); !errors.Is(err, ErrLockLost) {
		t.Fatalf("release = %v", err)
	}
}

func TestLostLockCancelsContext(t *testing.T) {
	t.Parallel()
	path := lockPath(t)
	now := time.Now()
	l, tick := holdLock(t, path, now)
	if err := l.Context().Err(); err != nil || l.Err() != nil {
		t.Fatalf("fresh lock ctx %v err %v", err, l.Err())
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	tick <- now.Add(lockRefresh)
	select {
	case <-l.Context().Done():
	case <-time.After(5 * time.Second):
		t.Fatal("context not cancelled after loss")
	}
	if !errors.Is(context.Cause(l.Context()), ErrLockLost) || !errors.Is(l.Err(), ErrLockLost) {
		t.Fatalf("cause %v err %v", context.Cause(l.Context()), l.Err())
	}
	if err := l.Release(); !errors.Is(err, ErrLockLost) {
		t.Fatalf("release = %v", err)
	}
}

func TestReleaseCancelsContext(t *testing.T) {
	t.Parallel()
	l, err := AcquireLock(lockPath(t), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if err := l.Release(); err != nil {
		t.Fatal(err)
	}
	if l.Context().Err() == nil {
		t.Fatal("context alive after release")
	}
}

func TestRefreshAfterLongPauseGivesUp(t *testing.T) {
	t.Parallel()
	path := lockPath(t)
	start := time.Now()
	l, err := AcquireLock(path, start)
	if err != nil {
		t.Fatal(err)
	}
	if err := l.refresh(start.Add(lockGrace + time.Second)); !errors.Is(err, ErrLockLost) {
		t.Fatalf("refresh = %v", err)
	}
	info, err := readInfo(path)
	if err != nil || !info.Time.Equal(start.UTC()) {
		t.Fatalf("lock rewritten %+v %v", info, err)
	}
	if err := l.Release(); err != nil {
		t.Fatal(err)
	}
}

func TestReaderDoesNotBreakHolder(t *testing.T) {
	t.Parallel()
	path := lockPath(t)
	now := time.Now()
	l, tick := holdLock(t, path, now)
	f, err := os.Open(filepath.Clean(path))
	if err != nil {
		t.Fatal(err)
	}
	beat(tick, now.Add(lockRefresh))
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	beat(tick, now.Add(2*lockRefresh))
	if err := l.Context().Err(); err != nil {
		t.Fatalf("holder cancelled by a reader: %v", context.Cause(l.Context()))
	}
	if err := l.Release(); err != nil {
		t.Fatal(err)
	}
}
