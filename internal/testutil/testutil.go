package testutil

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/elliot40404/creds/internal/crypto"
)

var ErrNoAnswer = errors.New("fake: no answer queued")

func Pop[T any](q *[]T) (T, error) {
	var zero T
	if len(*q) == 0 {
		return zero, ErrNoAnswer
	}
	v := (*q)[0]
	*q = (*q)[1:]
	return v, nil
}

func Main(m *testing.M) {
	os.Exit(Run(m))
}

func Run(m *testing.M) int {
	dir, err := os.MkdirTemp("", "creds-git-")
	if err != nil {
		return 1
	}
	defer func() { _ = os.RemoveAll(dir) }()
	global := filepath.Join(dir, "gitconfig")
	if err := os.WriteFile(global, nil, 0o600); err != nil {
		return 1
	}
	if os.Setenv("GIT_CONFIG_GLOBAL", global) != nil || os.Setenv("GIT_CONFIG_NOSYSTEM", "1") != nil {
		return 1
	}
	if addPluginDir(dir) != nil {
		return 1
	}
	return m.Run()
}

func Identity(tb testing.TB) *crypto.Identity {
	tb.Helper()
	id, err := crypto.NewIdentity()
	if err != nil {
		tb.Fatal(err)
	}
	return id
}

func Keys(tb testing.TB) (*crypto.Identity, []byte) {
	tb.Helper()
	id := Identity(tb)
	key, err := id.MACKey()
	if err != nil {
		tb.Fatal(err)
	}
	return id, key
}

func Git(tb testing.TB, dir string, args ...string) string {
	tb.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		tb.Skip("git not installed")
	}
	base := []string{"-c", "commit.gpgsign=false", "-c", "core.hooksPath=" + os.DevNull, "-c", "user.name=creds test", "-c", "user.email=test@localhost"}
	cmd := exec.Command("git", append(base, args...)...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		tb.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func BareRemote(tb testing.TB, branch string) string {
	tb.Helper()
	dir := filepath.Join(tb.TempDir(), "remote.git")
	Git(tb, filepath.Dir(dir), "init", "-q", "--bare", "-b", branch, dir)
	return dir
}

func PushChange(tb testing.TB, url, branch, msg string, change func(dir string)) {
	tb.Helper()
	dir := filepath.Join(tb.TempDir(), "work")
	Git(tb, filepath.Dir(dir), "clone", "-q", "--", url, dir)
	change(dir)
	Git(tb, dir, "add", "-A")
	Git(tb, dir, "commit", "-q", "-m", msg)
	Git(tb, dir, "push", "-q", "origin", "HEAD:"+branch)
}
