package app

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/elliot40404/creds/internal/anchor"
	"github.com/elliot40404/creds/internal/crypto"
	"github.com/elliot40404/creds/internal/gitsync"
)

const (
	commitMessage = "update vault"
	gitDir        = ".git"
)

func (s *Service) repo(ctx context.Context) (*gitsync.Repo, error) {
	dir := s.Paths.Vault()
	_, err := os.Stat(filepath.Join(dir, gitDir))
	if errors.Is(err, fs.ErrNotExist) {
		r, err := gitsync.InitRepo(ctx, dir)
		if err != nil {
			return nil, err
		}
		return s.signing(r), nil
	}
	if err != nil {
		return nil, err
	}
	return s.signing(gitsync.Open(dir)), nil
}

func (s *Service) signing(r *gitsync.Repo) *gitsync.Repo {
	r.Configure(s.CurrentConfig().Sync)
	return r
}

func (s *Service) commit(id *crypto.Identity) error {
	changed, err := s.commitOnly(id)
	if changed {
		s.afterWrite()
	}
	return err
}

func (s *Service) commitOnly(id *crypto.Identity) (bool, error) {
	ctx := context.Background()
	r, err := s.repo(ctx)
	if err != nil {
		return false, err
	}
	m, err := s.refreshManifest(ctx, r, id)
	if err != nil {
		return false, err
	}
	changed, err := r.Commit(ctx, commitMessage)
	if err != nil {
		return changed, err
	}
	return changed, anchor.Record(s.Paths.Anchor(), m)
}
