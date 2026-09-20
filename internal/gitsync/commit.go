package gitsync

import (
	"context"
	"strings"

	"github.com/elliot40404/creds/internal/vaultfiles"
)

func (r *Repo) Commit(ctx context.Context, msg string) (bool, error) {
	if err := r.stage(ctx); err != nil {
		return false, err
	}
	changes, err := r.staged(ctx)
	if err != nil || len(changes) == 0 {
		return false, err
	}
	if err := checkStaged(changes); err != nil {
		return false, err
	}
	if _, err := r.git.Run(ctx, "commit", "-q", "--no-verify", "-m", msg); err != nil {
		return false, err
	}
	return true, nil
}

func (r *Repo) stage(ctx context.Context) error {
	out, err := r.git.Run(ctx, "ls-files", "-z", "--cached", "--others", "--exclude-standard")
	if err != nil {
		return err
	}
	var paths []string
	seen := map[string]bool{}
	for p := range strings.SplitSeq(string(out), "\x00") {
		if vaultfiles.Allowed(p) && !seen[p] {
			seen[p] = true
			paths = append(paths, p)
		}
	}
	if len(paths) == 0 {
		return nil
	}
	if err := r.checkWorktree(paths); err != nil {
		return err
	}
	args := []string{"add", "-A", "--"}
	for _, p := range paths {
		args = append(args, ":(literal)"+p)
	}
	_, err = r.git.Run(ctx, args...)
	return err
}

func (r *Repo) staged(ctx context.Context) ([]change, error) {
	out, err := r.git.Run(ctx, "diff", "--cached", "--name-status", "-z", "--no-renames")
	if err != nil {
		return nil, err
	}
	parts := strings.Split(strings.TrimSuffix(string(out), "\x00"), "\x00")
	var changes []change
	for i := 0; i+1 < len(parts); i += 2 {
		changes = append(changes, change{status: parts[i], path: parts[i+1]})
	}
	return changes, nil
}
