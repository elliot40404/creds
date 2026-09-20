package gh

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"strings"
	"time"

	"github.com/elliot40404/creds/internal/detach"
)

const runTimeout = 2 * time.Minute

var (
	ErrBadRepoName = errors.New("invalid repository name")
	ErrNoHost      = errors.New("gh is not installed or not logged in")
	ErrRepoExists  = errors.New("repository already exists")
)

type Client struct {
	Run  func(ctx context.Context, args ...string) ([]byte, error)
	Look func(file string) (string, error)
}

func New() *Client { return &Client{} }

func Look(file string) (string, error) {
	return exec.LookPath(file)
}

func (g *Client) run(ctx context.Context, args ...string) ([]byte, error) {
	if g.Run != nil {
		return g.Run(ctx, args...)
	}
	ctx, cancel := context.WithTimeout(ctx, runTimeout)
	defer cancel()
	var out, errOut bytes.Buffer
	cmd := exec.CommandContext(ctx, "gh")
	cmd.Args = append(cmd.Args, args...)
	cmd.Stdout, cmd.Stderr = &out, &errOut
	detach.Hide(cmd)
	if err := cmd.Run(); err != nil {
		return out.Bytes(), ghError(args, errOut.String(), err)
	}
	return out.Bytes(), nil
}

func ghError(args []string, stderr string, err error) error {
	msg := strings.TrimSpace(stderr)
	if msg == "" {
		return err
	}
	return errors.New("gh " + strings.Join(args, " ") + ": " + msg)
}

func (g *Client) look(file string) (string, error) {
	if g.Look != nil {
		return g.Look(file)
	}
	return exec.LookPath(file)
}

func (g *Client) Available(ctx context.Context) error {
	if _, err := g.look("gh"); err != nil {
		return ErrNoHost
	}
	if !g.ok(ctx, "auth", "status") {
		return ErrNoHost
	}
	return nil
}

func (g *Client) RepoExists(ctx context.Context, name string) (bool, error) {
	if err := checkRepoName(name); err != nil {
		return false, err
	}
	if _, err := g.run(ctx, "repo", "view", name, "--json", "name"); err != nil {
		if notFound(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func notFound(err error) bool {
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "could not resolve to a repository")
}

func (g *Client) ok(ctx context.Context, args ...string) bool {
	_, err := g.run(ctx, args...)
	return err == nil
}

func (g *Client) Create(ctx context.Context, name string) (string, error) {
	if err := checkRepoName(name); err != nil {
		return "", err
	}
	exists, err := g.RepoExists(ctx, name)
	if err != nil {
		return "", err
	}
	if exists {
		return "", ErrRepoExists
	}
	if _, err := g.run(ctx, "repo", "create", name, "--private"); err != nil {
		return "", err
	}
	return g.url(ctx, name)
}

func (g *Client) url(ctx context.Context, name string) (string, error) {
	out, err := g.run(ctx, "repo", "view", name, "--json", "sshUrl", "--jq", ".sshUrl")
	if err != nil {
		return "", err
	}
	url := strings.TrimSpace(string(out))
	if url == "" {
		return "", errors.New("gh returned no url for " + name)
	}
	return url, nil
}

func (g *Client) Repos(ctx context.Context) ([]string, error) {
	out, err := g.run(ctx, "repo", "list", "--limit", "100", "--json", "sshUrl", "--jq", ".[].sshUrl")
	if err != nil {
		return nil, err
	}
	var urls []string
	for line := range strings.Lines(strings.TrimSpace(string(out))) {
		if line = strings.TrimSpace(line); line != "" {
			urls = append(urls, line)
		}
	}
	return urls, nil
}

func checkRepoName(name string) error {
	if name == "" || len(name) > 100 || strings.HasPrefix(name, "-") {
		return ErrBadRepoName
	}
	for _, r := range name {
		ok := r == '-' || r == '_' || r == '.' || r >= '0' && r <= '9' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z'
		if !ok {
			return ErrBadRepoName
		}
	}
	return nil
}
