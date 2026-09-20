package gitsync

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

func lockPath(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "home", "sync.lock")
}

func TestLockContention(t *testing.T) {
	t.Parallel()
	path := lockPath(t)
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	l, err := AcquireLock(path, now)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil || !strings.Contains(string(data), strconv.Itoa(os.Getpid())) {
		t.Fatalf("lock content %q %v", data, err)
	}
	if !Locked(path, now.Add(time.Minute)) {
		t.Fatal("not locked")
	}
	if _, err := AcquireLock(path, now.Add(LockStale-time.Second)); !errors.Is(err, ErrLocked) {
		t.Fatalf("second acquire = %v", err)
	}
	if err := l.Release(); err != nil {
		t.Fatal(err)
	}
	if Locked(path, now) {
		t.Fatal("still locked")
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("lock file left: %v", err)
	}
	l, err = AcquireLock(path, now)
	if err != nil {
		t.Fatal(err)
	}
	if err := l.Release(); err != nil {
		t.Fatal(err)
	}
}

func TestStaleLockTakenOver(t *testing.T) {
	t.Parallel()
	path := lockPath(t)
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	old, err := AcquireLock(path, now)
	if err != nil {
		t.Fatal(err)
	}
	later := now.Add(LockStale)
	if Locked(path, later) {
		t.Fatal("stale lock reported held")
	}
	fresh, err := AcquireLock(path, later)
	if err != nil {
		t.Fatalf("takeover: %v", err)
	}
	if err := old.Release(); err != nil {
		t.Fatal(err)
	}
	if !Locked(path, later) {
		t.Fatal("old holder removed the new lock")
	}
	if err := fresh.Release(); err != nil {
		t.Fatal(err)
	}
}

func TestCorruptLockUsesModTime(t *testing.T) {
	t.Parallel()
	path := lockPath(t)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	if !Locked(path, now) {
		t.Fatal("fresh corrupt lock not held")
	}
	if _, err := AcquireLock(path, now); !errors.Is(err, ErrLocked) {
		t.Fatalf("acquire = %v", err)
	}
	l, err := AcquireLock(path, now.Add(LockStale+time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if err := l.Release(); err != nil {
		t.Fatal(err)
	}
}

func TestWaitLock(t *testing.T) {
	t.Parallel()
	path := lockPath(t)
	held, err := AcquireLock(path, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	if _, err := WaitLock(ctx, path, time.Now, 5*time.Millisecond, nil); !errors.Is(err, ErrLocked) {
		t.Fatalf("wait = %v", err)
	}
	done := make(chan error, 1)
	go func() {
		l, err := WaitLock(context.Background(), path, time.Now, 5*time.Millisecond, nil)
		if err == nil {
			err = l.Release()
		}
		done <- err
	}()
	time.Sleep(20 * time.Millisecond)
	if err := held.Release(); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func deadPID(t *testing.T) int {
	t.Helper()
	cmd := exec.Command("git", "--version")
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}
	return cmd.ProcessState.Pid()
}

func TestDeadOwnerLockIsStale(t *testing.T) {
	t.Parallel()
	path := lockPath(t)
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	data := fmt.Sprintf(`{"pid":%d,"time":%q}`, deadPID(t), now.Format(time.RFC3339Nano))
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	if Locked(path, now) {
		t.Fatal("dead owner lock reported held")
	}
	l, err := AcquireLock(path, now)
	if err != nil {
		t.Fatalf("takeover: %v", err)
	}
	if !Locked(path, now) {
		t.Fatal("live lock not held")
	}
	if err := l.Release(); err != nil {
		t.Fatal(err)
	}
}

func TestStaleTakeoverHasOneWinner(t *testing.T) {
	t.Parallel()
	path := lockPath(t)
	now := time.Now()
	stale := fmt.Sprintf(`{"pid":%d,"time":%q}`, deadPID(t), now.Format(time.RFC3339Nano))
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	wins := 0
	for range 50 {
		if err := os.WriteFile(path, []byte(stale), 0o600); err != nil {
			t.Fatal(err)
		}
		locks := contend(path, now, 4)
		if len(locks) > 1 {
			t.Fatalf("%d contenders took the stale lock", len(locks))
		}
		for _, l := range locks {
			wins++
			if err := l.Release(); err != nil {
				t.Fatal(err)
			}
		}
	}
	if wins == 0 {
		t.Fatal("stale lock never taken over")
	}
}

func contend(path string, now time.Time, n int) []*Lock {
	var mu sync.Mutex
	var wg sync.WaitGroup
	var won []*Lock
	start := make(chan struct{})
	for range n {
		wg.Go(func() {
			<-start
			if l, err := acquire(path, now, nil); err == nil {
				mu.Lock()
				won = append(won, l)
				mu.Unlock()
			}
		})
	}
	close(start)
	wg.Wait()
	return won
}
