package gitsync

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

const lockHelperEnv = "GITSYNC_LOCK_HELPER_PATH"

func TestLockHelperProcess(t *testing.T) {
	path := os.Getenv(lockHelperEnv)
	if path == "" {
		t.Skip("helper only")
	}
	if _, err := AcquireLock(path, time.Now()); err != nil {
		fmt.Println("error", err)
		os.Exit(1)
	}
	fmt.Println("held")
	_, _ = bufio.NewReader(os.Stdin).ReadString('\n')
	os.Exit(0)
}

func startLockHolder(t *testing.T, path string) *exec.Cmd {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := &exec.Cmd{Path: exe, Args: []string{exe, "-test.run=^TestLockHelperProcess$"}}
	cmd.Env = append(os.Environ(), lockHelperEnv+"="+path)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = stdin.Close()
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	})
	if line, err := bufio.NewReader(stdout).ReadString('\n'); err != nil || line != "held\n" {
		t.Fatalf("holder said %q %v", line, err)
	}
	return cmd
}

func mustAcquire(t *testing.T, path string, now time.Time) *Lock {
	t.Helper()
	l, err := AcquireLock(path, now)
	if err != nil {
		t.Fatal(err)
	}
	return l
}

func wantLocked(t *testing.T, path string, now time.Time) {
	t.Helper()
	if !Locked(path, now) {
		t.Fatal("Locked = false")
	}
	if l, err := AcquireLock(path, now); !errors.Is(err, ErrLocked) {
		if l != nil {
			_ = l.Release()
		}
		t.Fatalf("acquire = %v, want ErrLocked", err)
	}
}

func writeOldLock(t *testing.T, path string, pid int, at time.Time) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	data := fmt.Sprintf(`{"pid":%d,"time":%q}`, pid, at.Format(time.RFC3339Nano))
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestReleaseCancelsContext(t *testing.T) {
	t.Parallel()
	l := mustAcquire(t, lockPath(t), time.Now())
	if l.Context().Err() != nil {
		t.Fatal("fresh lock already done")
	}
	if err := l.Release(); err != nil {
		t.Fatal(err)
	}
	if l.Context().Err() == nil {
		t.Fatal("context alive after release")
	}
	if err := l.Release(); err != nil {
		t.Fatalf("second release: %v", err)
	}
}

func TestHolderKeepsLockPastStale(t *testing.T) {
	t.Parallel()
	path := lockPath(t)
	now := time.Now()
	l := mustAcquire(t, path, now)
	wantLocked(t, path, now.Add(10*LockStale))
	if err := l.Release(); err != nil {
		t.Fatal(err)
	}
}

func TestChildHolderBlocksUntilKilled(t *testing.T) {
	t.Parallel()
	path := lockPath(t)
	cmd := startLockHolder(t, path)
	wantLocked(t, path, time.Now())
	wantLocked(t, path, time.Now().Add(10*LockStale))
	if err := cmd.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	_ = cmd.Wait()
	if err := mustAcquire(t, path, time.Now()).Release(); err != nil {
		t.Fatal(err)
	}
}

func TestOldFormatLiveHolderBlocks(t *testing.T) {
	t.Parallel()
	path := lockPath(t)
	now := time.Now()
	writeOldLock(t, path, os.Getppid(), now)
	wantLocked(t, path, now)
	if err := mustAcquire(t, path, now.Add(LockStale)).Release(); err != nil {
		t.Fatal(err)
	}
}

func TestOldFormatDeadOrStaleDoesNotBlock(t *testing.T) {
	t.Parallel()
	now := time.Now()
	for name, info := range map[string]lockInfo{
		"dead pid":    {PID: deadPID(t), Time: now},
		"stale time":  {PID: os.Getppid(), Time: now.Add(-LockStale)},
		"future time": {PID: os.Getppid(), Time: now.Add(LockStale)},
		"own pid":     {PID: os.Getpid(), Time: now},
	} {
		path := lockPath(t)
		writeOldLock(t, path, info.PID, info.Time)
		if Locked(path, now) {
			t.Fatalf("%s: Locked = true", name)
		}
		if err := mustAcquire(t, path, now).Release(); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
	}
}

func TestReleaseRemovesOnlyItsOwnFile(t *testing.T) {
	t.Parallel()
	path := lockPath(t)
	now := time.Now()
	l := mustAcquire(t, path, now)
	if err := l.Release(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("sync.lock left: %v", err)
	}
	l = mustAcquire(t, path, now)
	writeOldLock(t, path, os.Getppid(), now)
	if err := l.Release(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("foreign sync.lock removed: %v", err)
	}
}

func TestProbesDoNotBlockAcquire(t *testing.T) {
	t.Parallel()
	path := lockPath(t)
	stop := make(chan struct{})
	var wg sync.WaitGroup
	for range 4 {
		wg.Go(func() {
			for {
				select {
				case <-stop:
					return
				default:
					Locked(path, time.Now())
					time.Sleep(time.Millisecond)
				}
			}
		})
	}
	defer func() { close(stop); wg.Wait() }()
	for range 20 {
		if err := mustAcquire(t, path, time.Now()).Release(); err != nil {
			t.Fatal(err)
		}
	}
}
