package gitsync

import (
	"context"
	"errors"
	"os/exec"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/elliot40404/creds/internal/config"
)

func needGit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
}

func TestRunCapturesStderr(t *testing.T) {
	t.Parallel()
	needGit(t)
	g := NewGit(t.TempDir())
	_, err := g.Run(context.Background(), "rev-parse", "--verify", "HEAD")
	var gerr *Error
	if !errors.As(err, &gerr) {
		t.Fatalf("want *Error, got %v", err)
	}
	if gerr.Stderr == "" || exitCode(err) <= 0 {
		t.Fatalf("stderr %q code %d", gerr.Stderr, gerr.ExitCode)
	}
	if !strings.Contains(err.Error(), "rev-parse") {
		t.Fatalf("error lacks args: %v", err)
	}
}

func TestRunForcedConfig(t *testing.T) {
	t.Parallel()
	needGit(t)
	g := NewGit(t.TempDir())
	cases := map[string]string{
		"user.name":            g.userName(),
		"user.email":           g.userEmail(),
		"core.autocrlf":        "false",
		"core.symlinks":        "false",
		"maintenance.auto":     "false",
		"protocol.allow":       "never",
		"protocol.https.allow": "always",
		"protocol.ssh.allow":   "always",
		"protocol.file.allow":  "always",
	}
	for key, want := range cases {
		out, err := g.Run(context.Background(), "config", "--get", key)
		if err != nil {
			t.Fatalf("%s: %v", key, err)
		}
		if got := strings.TrimSpace(string(out)); got != want {
			t.Fatalf("%s = %q want %q", key, got, want)
		}
	}
}

func TestRunEnv(t *testing.T) {
	t.Setenv("GIT_DIR", "/nowhere")
	t.Setenv("GIT_TERMINAL_PROMPT", "1")
	for _, kv := range env() {
		if strings.HasPrefix(kv, "GIT_DIR=") || kv == "GIT_TERMINAL_PROMPT=1" {
			t.Fatalf("leaked %s", kv)
		}
	}
	if !strings.Contains(strings.Join(env(), "\n"), "GIT_TERMINAL_PROMPT=0") {
		t.Fatal("prompt not disabled")
	}
}

func TestRunTimeout(t *testing.T) {
	t.Parallel()
	needGit(t)
	g := NewGit(t.TempDir())
	g.Timeout = time.Nanosecond
	_, err := g.Run(context.Background(), "version")
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("want deadline, got %v", err)
	}
}

func signFlags(args []string) []string {
	var out []string
	for i, a := range args {
		if a != "-c" || i+1 >= len(args) {
			continue
		}
		v := args[i+1]
		if strings.HasPrefix(v, "commit.gpgsign=") || strings.HasPrefix(v, "gpg.format=") || strings.HasPrefix(v, "user.signingkey=") {
			out = append(out, v)
		}
	}
	return out
}

func TestSignArgs(t *testing.T) {
	cases := map[string]struct {
		mode config.Sign
		key  string
		want []string
	}{
		"zero value is off": {"", "", []string{"commit.gpgsign=false"}},
		"off":               {config.SignOff, "", []string{"commit.gpgsign=false"}},
		"off ignores key":   {config.SignOff, "/k.pub", []string{"commit.gpgsign=false"}},
		"ssh":               {config.SignSSH, "", []string{"commit.gpgsign=true", "gpg.format=ssh"}},
		"ssh with key":      {config.SignSSH, "/k.pub", []string{"commit.gpgsign=true", "gpg.format=ssh", "user.signingkey=/k.pub"}},
		"inherit":           {config.SignInherit, "", nil},
		"inherit drops key": {config.SignInherit, "/k.pub", nil},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			g := NewGit("dir")
			g.sign, g.signKey = tc.mode, tc.key
			got := signFlags(g.args([]string{"commit"}))
			if !slices.Equal(got, tc.want) {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestSignsCommits(t *testing.T) {
	g := NewGit("dir")
	if g.signsCommits() {
		t.Fatal("zero value signs")
	}
	g.sign, g.signKey = config.SignSSH, ""
	if !g.signsCommits() {
		t.Fatal("ssh does not sign")
	}
	g.sign, g.signKey = config.SignInherit, ""
	if g.signsCommits() {
		t.Fatal("inherit must not pass -S to commit-tree, it is documented as unsigned")
	}
}

func TestForcedSigningConfigPerMode(t *testing.T) {
	t.Parallel()
	needGit(t)
	cases := map[string]struct {
		mode      config.Sign
		key       string
		gpgsign   string
		format    string
		signkeyIn bool
	}{
		"off":     {config.SignOff, "", "false", "", false},
		"ssh":     {config.SignSSH, "", "true", "ssh", false},
		"ssh key": {config.SignSSH, "/k.pub", "true", "ssh", true},
		"inherit": {config.SignInherit, "/k.pub", "", "", false},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			g := NewGit(t.TempDir())
			g.sign, g.signKey = tc.mode, tc.key
			check := func(key, want string) {
				t.Helper()
				out, err := g.Run(context.Background(), "config", "--get", key)
				got := strings.TrimSpace(string(out))
				if want == "" {
					if err == nil && got != "" {
						t.Fatalf("%s is set to %q, want unset", key, got)
					}
					return
				}
				if err != nil {
					t.Fatalf("%s: %v", key, err)
				}
				if got != want {
					t.Fatalf("%s = %q want %q", key, got, want)
				}
			}
			if tc.mode == config.SignInherit {
				if got := signFlags(g.args(nil)); got != nil {
					t.Fatalf("inherit forces %q, the user's own git config must decide", got)
				}
				return
			}
			check("commit.gpgsign", tc.gpgsign)
			check("gpg.format", tc.format)
			if tc.signkeyIn {
				check("user.signingkey", tc.key)
			}
		})
	}
}
