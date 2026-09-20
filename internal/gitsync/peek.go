package gitsync

import (
	"context"
	"os"
	"strings"
	"time"
)

const (
	PeekTimeout = 30 * time.Second
	peekFilter  = "--filter=blob:limit=1m"
)

func Peek(ctx context.Context, url string) ([]string, error) {
	if err := checkURL(url); err != nil {
		return nil, err
	}
	dir, err := os.MkdirTemp("", "creds-peek")
	if err != nil {
		return nil, err
	}
	defer func() { _ = os.RemoveAll(dir) }()
	g := &Git{Dir: dir, Timeout: PeekTimeout}
	if _, err := g.Run(ctx, "init", "-q", "-b", Branch); err != nil {
		return nil, err
	}
	refs, err := lsRemote(ctx, g, url)
	if err != nil || refs == "" {
		return nil, err
	}
	return peekFiles(ctx, g, url)
}

func lsRemote(ctx context.Context, g *Git, url string) (string, error) {
	out, err := g.Run(ctx, "ls-remote", "--", url)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func peekFiles(ctx context.Context, g *Git, url string) ([]string, error) {
	if _, err := g.Run(ctx, "fetch", "-q", "--depth", "1", "--no-tags", peekFilter, "--", url, "HEAD"); err != nil {
		return nil, err
	}
	out, err := g.RunLimit(ctx, maxTreeOutput, "ls-tree output", "ls-tree", "-r", "--name-only", "-z", "FETCH_HEAD")
	if err != nil {
		return nil, err
	}
	names := strings.Split(strings.TrimRight(string(out), "\x00"), "\x00")
	if len(names) == 1 && names[0] == "" {
		return nil, nil
	}
	return names, nil
}
