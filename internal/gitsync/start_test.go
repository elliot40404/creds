package gitsync

import (
	"context"
	"errors"
	"os/exec"
	"strings"
	"testing"
)

func exitErr(t *testing.T) error {
	t.Helper()
	err := exec.Command("git", "rev-parse", "--verify", "refs/heads/creds-no-such-branch").Run()
	if _, ok := errors.AsType[*exec.ExitError](err); !ok {
		t.Skipf("no exit error from git: %v", err)
	}
	return err
}

func TestStartFailedClassifies(t *testing.T) {
	t.Parallel()
	exit := exitErr(t)
	cases := []struct {
		name string
		err  *Error
		want bool
	}{
		{"dll init", &Error{Err: exit, ExitCode: 0xc0000142}, true},
		{"dll init with stderr", &Error{Err: exit, ExitCode: 0xc0000142, Stderr: "fatal: nope"}, false},
		{"plain exit", &Error{Err: exit, ExitCode: 1}, false},
		{"signal", &Error{Err: exit, ExitCode: -1}, false},
		{"spawn refused", &Error{Err: errors.New("fork/exec: resource temporarily unavailable"), ExitCode: -1}, true},
		{"git missing", &Error{Err: &exec.Error{Name: "git", Err: exec.ErrNotFound}, ExitCode: -1}, false},
		{"timeout", &Error{Err: context.DeadlineExceeded, ExitCode: -1}, false},
		{"canceled", &Error{Err: context.Canceled, ExitCode: -1}, false},
	}
	for _, c := range cases {
		if got := startFailed(c.err); got != c.want {
			t.Errorf("%s: startFailed = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestStartErrorTextAndMatch(t *testing.T) {
	t.Parallel()
	e := &Error{Args: []string{"rev-parse", "HEAD"}, ExitCode: 0xc0000142, Start: true, Err: errors.New("exit status 0xc0000142")}
	if !errors.Is(e, ErrGitStart) {
		t.Fatal("want ErrGitStart match")
	}
	msg := e.Error()
	if !strings.Contains(msg, "git could not start") || strings.Contains(msg, "0xc0000142") {
		t.Fatalf("msg %q", msg)
	}
	plain := &Error{Args: []string{"push"}, ExitCode: 1, Stderr: "rejected", Err: errors.New("exit status 1")}
	if errors.Is(plain, ErrGitStart) || !strings.Contains(plain.Error(), "rejected") {
		t.Fatalf("msg %q", plain.Error())
	}
}

func TestRetryStartRunsTwiceOnce(t *testing.T) {
	t.Parallel()
	start := &Error{Args: []string{"rev-parse"}, Start: true, Err: errors.New("boom")}
	tries := 0
	err := retryStart(t.Context(), true, func() error {
		tries++
		return start
	})
	if tries != 2 || !errors.Is(err, ErrGitStart) {
		t.Fatalf("tries %d err %v", tries, err)
	}
	tries = 0
	err = retryStart(t.Context(), true, func() error {
		tries++
		if tries == 1 {
			return start
		}
		return nil
	})
	if tries != 2 || err != nil {
		t.Fatalf("tries %d err %v", tries, err)
	}
	tries = 0
	if err := retryStart(t.Context(), false, func() error { tries++; return start }); tries != 1 || !errors.Is(err, ErrGitStart) {
		t.Fatalf("tries %d err %v", tries, err)
	}
	tries = 0
	if err := retryStart(t.Context(), true, func() error { tries++; return errors.New("other") }); tries != 1 || err == nil {
		t.Fatalf("tries %d err %v", tries, err)
	}
}
