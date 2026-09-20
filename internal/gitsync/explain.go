package gitsync

import (
	"errors"
	"strings"
)

var (
	ErrUnreachable  = errors.New("cannot reach the git host")
	ErrAuthFailed   = errors.New("git host refused the login")
	ErrRepoNotFound = errors.New("remote repository not found")
	ErrDenied       = errors.New("no permission to use the remote repository")
	ErrSignFailed   = errors.New("could not sign the vault commit")
)

var causes = []struct {
	err  error
	hits []string
}{
	{ErrGitStart, []string{strings.ToLower(ErrGitStart.Error())}},
	{ErrSignFailed, []string{
		"failed to sign the data", "user.signingkey needs to be set", "error: load key",
		"cannot run gpg", "gpg.format", "unsupported value for gpg.format",
		"couldn't load public key", "failed to write commit object", "no signing key",
		"error: cannot run ssh-keygen",
	}},
	{ErrUnreachable, []string{
		"could not resolve host", "could not resolve hostname", "temporary failure in name resolution",
		"connection refused", "connection timed out", "operation timed out", "network is unreachable",
		"no route to host", "failed to connect", "couldn't connect to server",
	}},
	{ErrAuthFailed, []string{
		"authentication failed", "invalid username or password", "permission denied (publickey",
		"could not read username", "could not read password", "host key verification failed",
	}},
	{ErrRepoNotFound, []string{"repository not found", "does not appear to be a git repository", "returned error: 404"}},
	{ErrDenied, []string{"returned error: 403", "write access to repository not granted", "access denied", "permission to "}},
}

func Cause(text string) error {
	low := strings.ToLower(text)
	for _, c := range causes {
		for _, h := range c.hits {
			if strings.Contains(low, h) {
				return c.err
			}
		}
	}
	return nil
}

func Detail(text string) string {
	line, _, _ := strings.Cut(strings.TrimSpace(text), "\n")
	if rest, ok := strings.CutPrefix(line, "git "); ok {
		if _, msg, found := strings.Cut(rest, ": "); found {
			return strings.TrimSpace(msg)
		}
	}
	return strings.TrimSpace(line)
}
