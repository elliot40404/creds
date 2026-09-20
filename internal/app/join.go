package app

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/elliot40404/creds/internal/fsutil"
	"github.com/elliot40404/creds/internal/gitsync"
	"github.com/elliot40404/creds/internal/vault"
)

var ErrJoinUndone = errors.New("join undone, local vault removed")

func (s *Service) Join(url string) error {
	if err := s.CheckFresh(); err != nil {
		return err
	}
	if err := s.sessions().Delete(); err != nil {
		return err
	}
	if _, err := gitsync.Clone(context.Background(), url, s.Paths.Vault()); err != nil {
		return err
	}
	if err := fsutil.SecureTree(s.Paths.Vault()); err != nil {
		return err
	}
	if err := s.verifyJoin(); err != nil {
		return fmt.Errorf("%w: %w", ErrJoinUndone, errors.Join(err, os.RemoveAll(s.Paths.Vault()), s.sessions().Delete()))
	}
	now := s.now().UTC()
	return gitsync.SaveState(s.Paths.State(), gitsync.State{LastPull: now, LastSync: now, LastResult: gitsync.FastForwarded.String()})
}

func (s *Service) verifyJoin() error {
	id, err := s.identity()
	if err != nil {
		return err
	}
	v, err := vault.Load(s.Paths.Vault(), id)
	if err != nil {
		return err
	}
	v.Close()
	return s.trustOnFirstUse(id)
}
