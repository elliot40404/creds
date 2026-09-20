package gitsync

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/elliot40404/creds/internal/fsutil"
)

func Clone(ctx context.Context, url, dir string) (*Repo, error) {
	if err := checkURL(url); err != nil {
		return nil, err
	}
	dir, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	if err := fsutil.CheckAbsent(dir); err != nil {
		return nil, fmt.Errorf("clone into %s: %w", dir, err)
	}
	if err := fsutil.EnsureDir(dir); err != nil {
		return nil, err
	}
	r, err := clone(ctx, url, dir)
	if err != nil {
		return nil, errors.Join(err, os.RemoveAll(dir))
	}
	return r, nil
}

func clone(ctx context.Context, url, dir string) (*Repo, error) {
	if _, err := NewGit(filepath.Dir(dir)).Run(ctx, "clone", "-q", "--no-checkout", "--", url, dir); err != nil {
		return nil, err
	}
	r := Open(dir)
	if err := r.configure(ctx); err != nil {
		return nil, err
	}
	if _, err := r.git.Run(ctx, "symbolic-ref", "HEAD", localRef); err != nil {
		return nil, err
	}
	remote, err := r.rev(ctx, remoteRef)
	if err != nil {
		return nil, err
	}
	if remote != "" {
		if err := r.checkout(ctx, remote); err != nil {
			return nil, err
		}
	}
	return r, r.ensureIgnore()
}

func (r *Repo) checkout(ctx context.Context, rev string) error {
	if _, _, err := r.revTrees(ctx, "", rev); err != nil {
		return err
	}
	_, err := r.git.Run(ctx, "reset", "-q", "--hard", rev)
	return err
}
