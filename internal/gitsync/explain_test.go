package gitsync

import (
	"errors"
	"testing"
)

func TestCauseMapsGitFailures(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		stderr string
		want   error
	}{
		{"fatal: unable to access 'https://h/v.git/': Could not resolve host: h", ErrUnreachable},
		{"ssh: connect to host h port 22: Connection refused\nfatal: Could not read from remote repository.", ErrUnreachable},
		{"fatal: unable to access 'https://h/v.git/': Failed to connect to h port 443 after 21 ms", ErrUnreachable},
		{"remote: Invalid username or password.\nfatal: Authentication failed for 'https://h/v.git/'", ErrAuthFailed},
		{"git@h: Permission denied (publickey).\nfatal: Could not read from remote repository.", ErrAuthFailed},
		{"fatal: could not read Username for 'https://h': terminal prompts disabled", ErrAuthFailed},
		{"remote: Repository not found.\nfatal: repository 'https://h/v.git/' not found", ErrRepoNotFound},
		{"fatal: '/tmp/gone' does not appear to be a git repository", ErrRepoNotFound},
		{"remote: Permission to you/v.git denied to other.\nfatal: unable to access 'https://h/v.git/': The requested URL returned error: 403", ErrDenied},
		{"fatal: bad object HEAD", nil},
	} {
		if got := Cause(tc.stderr); !errors.Is(got, tc.want) {
			t.Errorf("Cause(%q) = %v, want %v", tc.stderr, got, tc.want)
		}
		err := error(&Error{Args: []string{"fetch", "origin"}, Stderr: tc.stderr, Err: errors.New("exit status 128")})
		if tc.want != nil && !errors.Is(err, tc.want) {
			t.Errorf("errors.Is(%q, %v) = false", tc.stderr, tc.want)
		}
		if errors.Is(err, ErrGitStart) {
			t.Errorf("%q matched ErrGitStart", tc.stderr)
		}
	}
}

func TestCauseGitStart(t *testing.T) {
	t.Parallel()
	err := &Error{Args: []string{"fetch"}, Start: true, Err: errors.New("denied")}
	if !errors.Is(Cause(err.Error()), ErrGitStart) || !errors.Is(err, ErrGitStart) || errors.Is(err, ErrUnreachable) {
		t.Fatalf("start error %q", err)
	}
}

func TestDetailDropsGitCommand(t *testing.T) {
	t.Parallel()
	for in, want := range map[string]string{
		"git fetch -q origin +refs/heads/master:refs/remotes/origin/master: fatal: bad object HEAD\nmore": "fatal: bad object HEAD",
		"gitsync: parse state: bad json": "gitsync: parse state: bad json",
		"":                               "",
	} {
		if got := Detail(in); got != want {
			t.Errorf("Detail(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestCauseSpotsSigningFailures(t *testing.T) {
	cases := []string{
		"error: gpg failed to sign the data\nfatal: failed to write commit object",
		`error: Load key "/home/me/.ssh/missing": No such file or directory`,
		"error: Load key \"/k\": error in libcrypto\nfatal: failed to write commit object",
		"fatal: user.signingkey needs to be set for ssh signing",
		"error: cannot run gpg: No such file or directory",
		"error: unsupported value for gpg.format: nope",
		`error: Couldn't load public key /tmp/missing.pub: No such file or directory?

fatal: failed to write commit object`,
	}
	for _, text := range cases {
		if got := Cause(text); !errors.Is(got, ErrSignFailed) {
			t.Fatalf("%q mapped to %v", text, got)
		}
		if got := Cause(redact(text)); !errors.Is(got, ErrSignFailed) {
			t.Fatalf("after redact %q mapped to %v", text, got)
		}
	}
}

func TestCauseKeepsRemoteErrorsSeparate(t *testing.T) {
	if got := Cause("fatal: could not resolve host: example.com"); !errors.Is(got, ErrUnreachable) {
		t.Fatalf("got %v", got)
	}
	if got := Cause("remote: Permission to me/vault.git denied"); !errors.Is(got, ErrDenied) {
		t.Fatalf("got %v", got)
	}
}
