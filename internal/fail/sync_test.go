package fail

import (
	"errors"
	"testing"

	"github.com/elliot40404/creds/internal/gitsync"
)

func TestSyncTextMapsGitFailures(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		raw, msg string
		hint     bool
	}{
		{"git fetch -q origin: fatal: unable to access 'https://h/': Could not resolve host: h", gitsync.ErrUnreachable.Error(), true},
		{"git fetch origin: fatal: Authentication failed for 'https://h/v.git/'", gitsync.ErrAuthFailed.Error(), true},
		{"git fetch origin: remote: Repository not found.", gitsync.ErrRepoNotFound.Error(), true},
		{"git push origin: The requested URL returned error: 403", gitsync.ErrDenied.Error(), true},
		{"git fetch origin: fatal: bad object HEAD", "fatal: bad object HEAD", false},
	} {
		msg, hint := SyncText(tc.raw)
		if msg != tc.msg || (hint != "") != tc.hint {
			t.Errorf("SyncText(%q) = %q, %q", tc.raw, msg, hint)
		}
	}
}

func TestClassifyGitFailure(t *testing.T) {
	t.Parallel()
	err := &gitsync.Error{Args: []string{"fetch", "origin"}, Stderr: "fatal: Could not resolve host: h", Err: errors.New("exit status 128")}
	e := Classify(err)
	if e.Msg != gitsync.ErrUnreachable.Error() || e.Hint == "" {
		t.Fatalf("classified %q %q", e.Msg, e.Hint)
	}
}
