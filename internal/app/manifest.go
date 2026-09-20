package app

import (
	"context"
	"strings"

	"github.com/elliot40404/creds/internal/anchor"
	"github.com/elliot40404/creds/internal/crypto"
	"github.com/elliot40404/creds/internal/gitsync"
	"github.com/elliot40404/creds/internal/manifest"
)

func (s *Service) refreshManifest(ctx context.Context, r *gitsync.Repo, id *crypto.Identity) (manifest.Manifest, error) {
	parents, err := manifestParents(ctx, r, id)
	if err != nil {
		return manifest.Manifest{}, err
	}
	return manifest.Write(s.Paths.Vault(), id, parents)
}

func manifestParents(ctx context.Context, r *gitsync.Repo, id *crypto.Identity) ([]manifest.Manifest, error) {
	prev, ok, err := r.HeadManifest(ctx, id)
	if err != nil || !ok {
		return nil, err
	}
	return []manifest.Manifest{prev}, nil
}

func (s *Service) trustOnFirstUse(id *crypto.Identity) error {
	m, err := manifest.Verify(s.Paths.Vault(), id)
	if err != nil {
		return err
	}
	return anchor.Record(s.Paths.Anchor(), m)
}

func shortID(id string) string {
	const n = 12
	if len(id) <= n {
		return id
	}
	return strings.ToLower(id[:n])
}
