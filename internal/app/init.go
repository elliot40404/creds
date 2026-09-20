package app

import (
	"github.com/elliot40404/creds/internal/crypto"
	"github.com/elliot40404/creds/internal/fsutil"
	"github.com/elliot40404/creds/internal/vault"
	"github.com/elliot40404/creds/internal/vaultfiles"
)

func (s *Service) Init() error {
	if err := s.CheckFresh(); err != nil {
		return err
	}
	defer s.checkPerms()
	pw, err := s.newPassword()
	if err != nil {
		return err
	}
	code, err := s.confirmRecovery()
	if err != nil {
		return err
	}
	id, err := crypto.NewIdentity()
	if err != nil {
		return err
	}
	if err := s.writeIdentity(id, pw, code); err != nil {
		return err
	}
	v, err := vault.Init(s.Paths.Vault(), id)
	if err != nil {
		return err
	}
	v.Close()
	if err := s.commit(id); err != nil {
		return err
	}
	return s.sessions().Save(id.String())
}

func (s *Service) CheckFresh() error {
	ok, err := s.hasVault()
	if ok {
		return ErrVaultExists
	}
	if err != nil {
		return err
	}
	if err := s.checkNoTraces(); err != nil {
		return err
	}
	return s.confirmStray()
}

func (s *Service) confirmRecovery() (string, error) {
	code, err := crypto.NewRecoveryCode()
	if err != nil {
		return "", err
	}
	if err := s.Prompter.Show("Recovery code", code); err != nil {
		return "", err
	}
	return Retry(s.Prompter, func() (string, error) {
		typed, err := s.Prompter.Input("Type the recovery code", "")
		if err != nil {
			return "", err
		}
		if crypto.CanonicalRecoveryCode(typed) != code {
			return "", ErrRecoveryMismatch
		}
		return code, nil
	}, ErrRecoveryMismatch)
}

func (s *Service) writeIdentity(id *crypto.Identity, pw, code string) error {
	if err := fsutil.EnsureDir(s.Paths.Vault()); err != nil {
		return err
	}
	if err := s.wrapTo(vaultfiles.PasswordFile, id, pw); err != nil {
		return err
	}
	return s.wrapTo(vaultfiles.RecoveryFile, id, code)
}

func (s *Service) wrapTo(name string, id *crypto.Identity, secret string) error {
	data, err := crypto.WrapIdentity(id, secret, s.logN())
	if err != nil {
		return err
	}
	return fsutil.WriteFileAtomic(s.vaultFile(name), data)
}
