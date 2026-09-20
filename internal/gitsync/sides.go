package gitsync

import (
	"context"
	"fmt"
	"slices"

	"github.com/elliot40404/creds/internal/vault"
)

func (s *Syncer) ConflictSides(ctx context.Context, path string) (*vault.Entry, *vault.Entry, error) {
	r := s.Repo
	local, remote, err := r.heads(ctx)
	if err != nil {
		return nil, nil, err
	}
	trees, err := r.mergeTrees(ctx, local, remote)
	if err != nil {
		return nil, nil, err
	}
	sets, err := r.bucketSets(ctx, trees)
	if err != nil {
		return nil, nil, err
	}
	_, conflicts, err := vault.MergeVault(s.Identity, sets[0], sets[1], sets[2], nil)
	if err != nil {
		return nil, nil, err
	}
	for _, c := range conflicts {
		if slices.Contains(c.Paths, path) {
			return c.Ours, c.Theirs, nil
		}
	}
	return nil, nil, fmt.Errorf("%w: %s", ErrNoConflict, path)
}
