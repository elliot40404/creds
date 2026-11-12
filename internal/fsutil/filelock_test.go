package fsutil

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

const holdEnv = "FSUTIL_HOLD_PATH"

func TestHoldHelperProcess(t *testing.T) {
	path := os.Getenv(holdEnv)
	if path == "" {
		t.Skip("helper only")
	}
	if _, err := TryHold(path); err != nil {
		fmt.Println("error", err)
		os.Exit(1)
	}
	fmt.Println("held")
	_, _ = bufio.NewReader(os.Stdin).ReadString('\n')
	os.Exit(0)
}

func lockPath(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "x.lock")
}

func mustHold(t *testing.T, path string) *Held {
	t.Helper()
	h, err := TryHold(path)
	if err != nil {
		t.Fatal(err)
	}
	return h
}

func wantBusy(t *testing.T, path string) {
	t.Helper()
	if h, err := TryHold(path); !errors.Is(err, ErrBusy) {
		if h != nil {
			_ = h.Close()
		}
		t.Fatalf("want ErrBusy, got %v", err)
	}
}

func TestTryHoldBusyInProcess(t *testing.T) {
	t.Parallel()
	path := lockPath(t)
	h := mustHold(t, path)
	wantBusy(t, path)
	if err := h.Close(); err != nil {
		t.Fatal(err)
	}
	if err := h.Close(); err != nil {
		t.Fatalf("second close: %v", err)
	}
	if err := mustHold(t, path).Close(); err != nil {
		t.Fatal(err)
	}
}

func TestTryHoldOtherErrorsAreNotBusy(t *testing.T) {
	t.Parallel()
	_, err := TryHold(filepath.Join(t.TempDir(), "missing", "x.lock"))
	if err == nil || errors.Is(err, ErrBusy) {
		t.Fatalf("want a plain error, got %v", err)
	}
}

func TestWithFileLockWaitsThenRuns(t *testing.T) {
	t.Parallel()
	path := lockPath(t)
	h := mustHold(t, path)
	done := make(chan error, 1)
	go func() { done <- WithFileLock(path, func() error { return nil }) }()
	if err := h.Close(); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestTryHoldChildProcess(t *testing.T) {
	t.Parallel()
	path := lockPath(t)
	cmd, out := startHolder(t, path)
	if line, err := out.ReadString('\n'); err != nil || line != "held\n" {
		t.Fatalf("child said %q %v", line, err)
	}
	wantBusy(t, path)
	if err := cmd.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	_ = cmd.Wait()
	if err := mustHold(t, path).Close(); err != nil {
		t.Fatal(err)
	}
}

func startHolder(t *testing.T, path string) (*exec.Cmd, *bufio.Reader) {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := &exec.Cmd{Path: exe, Args: []string{exe, "-test.run=^TestHoldHelperProcess$"}}
	cmd.Env = append(os.Environ(), holdEnv+"="+path)
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
	return cmd, bufio.NewReader(stdout)
}
