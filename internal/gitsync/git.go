package gitsync

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/elliot40404/creds/internal/config"
	"github.com/elliot40404/creds/internal/detach"
)

const (
	defaultTimeout = 2 * time.Minute
	UserName       = "creds"
	UserEmail      = "creds@localhost"

	statusFailure    = 0xc0000000
	startRetryWait   = 200 * time.Millisecond
	globalLookupWait = 5 * time.Second
)

var ErrGitStart = errors.New("git could not start")

var dropEnv = []string{
	"GIT_DIR=", "GIT_WORK_TREE=", "GIT_INDEX_FILE=", "GIT_OBJECT_DIRECTORY=", "GIT_NAMESPACE=", "GIT_CEILING_DIRECTORIES=",
	"GIT_TERMINAL_PROMPT=", "GIT_MERGE_AUTOEDIT=", "LC_ALL=", "LANGUAGE=",
}

type Git struct {
	Dir     string
	Timeout time.Duration
	sign    config.Sign
	signKey string
	name    string
	email   string

	who *Committer
}

type Committer struct {
	Name        string
	Email       string
	NameGlobal  bool
	EmailGlobal bool
}

type Error struct {
	Args     []string
	ExitCode int
	Stderr   string
	Err      error
	Start    bool
}

func (e *Error) Error() string {
	msg := strings.TrimSpace(e.Stderr)
	switch {
	case e.Start:
		msg = ErrGitStart.Error() + ", the machine refused to launch the process"
	case msg == "":
		msg = e.Err.Error()
	}
	return redact(fmt.Sprintf("git %s: %s", strings.Join(e.Args, " "), msg))
}

func (e *Error) Unwrap() error { return e.Err }

func (e *Error) Is(target error) bool {
	if e.Start {
		return target == ErrGitStart
	}
	return target != ErrGitStart && target == Cause(e.Stderr)
}

func NewGit(dir string) *Git {
	return &Git{Dir: dir, Timeout: defaultTimeout}
}

func (g *Git) Run(ctx context.Context, args ...string) ([]byte, error) {
	var out bytes.Buffer
	err := g.run(ctx, nil, &out, args)
	return out.Bytes(), err
}

func (g *Git) RunLimit(ctx context.Context, limit int64, what string, args ...string) ([]byte, error) {
	var out bytes.Buffer
	err := g.run(ctx, nil, &limitWriter{w: &out, left: limit, what: what}, args)
	return out.Bytes(), err
}

func (g *Git) run(ctx context.Context, stdin io.Reader, stdout io.Writer, args []string) error {
	return retryStart(ctx, stdin == nil, func() error {
		return g.runOnce(ctx, stdin, stdout, args)
	})
}

func retryStart(ctx context.Context, replayable bool, once func() error) error {
	err := once()
	if !errors.Is(err, ErrGitStart) || !replayable {
		return err
	}
	if Wait(ctx, startRetryWait) != nil {
		return err
	}
	return once()
}

func Wait(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (g *Git) runOnce(ctx context.Context, stdin io.Reader, stdout io.Writer, args []string) error {
	if g.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, g.Timeout)
		defer cancel()
	}
	var stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, "git")
	cmd.Args = append(cmd.Args, g.args(args)...)
	cmd.Env = env()
	cmd.Stdin = stdin
	cmd.Stdout = stdout
	cmd.Stderr = &stderr
	detach.Hide(cmd)
	err := cmd.Run()
	if err == nil {
		return nil
	}
	gerr := &Error{Args: args, ExitCode: -1, Stderr: stderr.String(), Err: err}
	if ctxErr := ctx.Err(); ctxErr != nil {
		gerr.Err = ctxErr
	}
	if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
		gerr.ExitCode = exitErr.ExitCode()
	}
	gerr.Start = startFailed(gerr)
	return gerr
}

func startFailed(e *Error) bool {
	if e.Err == nil || errors.Is(e.Err, exec.ErrNotFound) || errors.Is(e.Err, context.Canceled) || errors.Is(e.Err, context.DeadlineExceeded) {
		return false
	}
	if _, ok := errors.AsType[*exec.ExitError](e.Err); !ok {
		return true
	}
	return e.ExitCode > 0 && int64(e.ExitCode) >= statusFailure && strings.TrimSpace(e.Stderr) == ""
}

func (g *Git) args(args []string) []string {
	base := []string{
		"-C", g.Dir,
		"-c", "core.autocrlf=false",
		"-c", "core.hooksPath=" + os.DevNull,
		"-c", "core.fsmonitor=false",
		"-c", "core.symlinks=false",
		"-c", "maintenance.auto=false",
		"-c", "user.name=" + g.userName(),
		"-c", "user.email=" + g.userEmail(),
		"-c", "protocol.allow=never",
		"-c", "protocol.https.allow=always",
		"-c", "protocol.ssh.allow=always",
		"-c", "protocol.file.allow=always",
		"-c", "transfer.fsckObjects=true",
	}
	return append(append(base, g.signArgs()...), args...)
}

func (g *Git) signArgs() []string {
	switch g.sign {
	case config.SignInherit:
		return nil
	case config.SignSSH:
		out := []string{"-c", "commit.gpgsign=true", "-c", "gpg.format=ssh"}
		if g.signKey != "" {
			out = append(out, "-c", "user.signingkey="+g.signKey)
		}
		return out
	}
	return []string{"-c", "commit.gpgsign=false"}
}

func (g *Git) userName() string {
	if g.name != "" {
		return g.name
	}
	return g.committer().Name
}

func (g *Git) userEmail() string {
	if g.email != "" {
		return g.email
	}
	return g.committer().Email
}

func (g *Git) committer() Committer {
	if g.who == nil {
		g.who = &Committer{Name: UserName, Email: UserEmail}
		if n := g.globalValue("user.name"); n != "" {
			g.who.Name, g.who.NameGlobal = n, true
		}
		if e := g.globalValue("user.email"); e != "" {
			g.who.Email, g.who.EmailGlobal = e, true
		}
	}
	return *g.who
}

func (g *Git) globalValue(key string) string {
	ctx, cancel := context.WithTimeout(context.Background(), globalLookupWait)
	defer cancel()
	var out bytes.Buffer
	if err := g.runOnce(ctx, nil, &out, []string{"config", "--global", "--get", key}); err != nil {
		return ""
	}
	v := strings.TrimSpace(out.String())
	if strings.ContainsAny(v, "<>\n") {
		return ""
	}
	return v
}

func (g *Git) signsCommits() bool {
	return g.sign == config.SignSSH
}

func env() []string {
	out := make([]string, 0, len(os.Environ())+4)
	for _, kv := range os.Environ() {
		if !dropped(kv) {
			out = append(out, kv)
		}
	}
	return append(out, "GIT_TERMINAL_PROMPT=0", "GIT_MERGE_AUTOEDIT=no", "LC_ALL=C", "LANGUAGE=C")
}

func dropped(kv string) bool {
	upper := strings.ToUpper(kv)
	for _, p := range dropEnv {
		if strings.HasPrefix(upper, p) {
			return true
		}
	}
	return false
}

func exitCode(err error) int {
	if gerr, ok := errors.AsType[*Error](err); ok {
		return gerr.ExitCode
	}
	return -1
}

func LookupCommitter(dir string) Committer {
	return NewGit(dir).committer()
}
