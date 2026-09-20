package app

import (
	"errors"
	"fmt"
	"io/fs"
	"os"

	"github.com/elliot40404/creds/internal/crypto"
	"github.com/elliot40404/creds/internal/format"
	"github.com/elliot40404/creds/internal/fsutil"
	"github.com/elliot40404/creds/internal/gitsync"
	"github.com/elliot40404/creds/internal/session"
	"github.com/elliot40404/creds/internal/vault"
	"github.com/elliot40404/creds/internal/vaultfiles"
)

const maxIdentitySize = 1 << 20

var ErrVaultIncomplete = errors.New("vault dir has files but no vault.json")

func (s *Service) Unlock() error {
	_, err := s.identity()
	return err
}

func (s *Service) Lock() error {
	return s.sessions().Delete()
}

func (s *Service) Recover() error {
	return s.resetSecret(vaultfiles.RecoveryFile, "Recovery code", crypto.CanonicalRecoveryCode)
}

func (s *Service) Passwd() error {
	return s.resetSecret(vaultfiles.PasswordFile, "Current master password", asTyped)
}

func (s *Service) resetSecret(file, prompt string, canon func(string) string) error {
	if err := s.requireVault(); err != nil {
		return err
	}
	id, err := s.askUnwrap(file, prompt, canon)
	if err != nil {
		return err
	}
	return s.replacePassword(id)
}

func (s *Service) replacePassword(id *crypto.Identity) error {
	pw, err := s.newPassword()
	if err != nil {
		return err
	}
	if err := s.withLock(func(lock *gitsync.Lock) (bool, error) { return s.writePassword(lock, id, pw) }); err != nil {
		return err
	}
	return s.sessions().Save(id.String())
}

func (s *Service) writePassword(lock *gitsync.Lock, id *crypto.Identity, pw string) (bool, error) {
	if err := s.wrapTo(vaultfiles.PasswordFile, id, pw); err != nil {
		return false, err
	}
	if err := lock.Err(); err != nil {
		return false, err
	}
	changed, err := s.commitOnly(id)
	if err != nil {
		return changed, err
	}
	return changed, gitsync.ClearPasswordChange(s.Paths.State())
}

func (s *Service) requireVault() error {
	ok, err := s.hasVault()
	if err != nil || ok {
		return err
	}
	if err := s.checkNoTraces(); err != nil {
		return err
	}
	return ErrNoVault
}

func (s *Service) checkNoTraces() error {
	for _, name := range append(vaultfiles.Required(), gitDir) {
		_, err := os.Lstat(s.vaultFile(name))
		if err == nil {
			return fmt.Errorf("%w: %s exists", ErrVaultIncomplete, name)
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return err
		}
	}
	return nil
}

func (s *Service) hasVault() (bool, error) {
	_, err := os.Lstat(s.vaultFile(vaultfiles.MetaFile))
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	return err == nil, err
}

func (s *Service) UnlockWith(pw string) error {
	_, err := s.loadIdentity(func() (*crypto.Identity, error) {
		return s.unwrap(vaultfiles.PasswordFile, pw)
	})
	return err
}

func (s *Service) identity() (*crypto.Identity, error) {
	return s.loadIdentity(func() (*crypto.Identity, error) {
		return s.askUnwrap(vaultfiles.PasswordFile, "Master password", asTyped)
	})
}

func (s *Service) loadIdentity(ask func() (*crypto.Identity, error)) (*crypto.Identity, error) {
	id, upgraded, err := s.openIdentity(ask)
	if err != nil || !upgraded {
		return id, err
	}
	return id, s.withLock(func(*gitsync.Lock) (bool, error) { return s.commitOnly(id) })
}

func (s *Service) migrate() (bool, error) {
	dir := s.Paths.Vault()
	pending, err := format.Pending(dir)
	if err != nil || !pending {
		return false, err
	}
	var from int
	err = s.withLock(func(*gitsync.Lock) (bool, error) {
		from, err = format.Migrate(dir)
		return false, err
	})
	if err != nil {
		return false, fmt.Errorf("migrate vault: %w", err)
	}
	return from < format.CurrentVersion, nil
}

func (s *Service) openIdentity(ask func() (*crypto.Identity, error)) (*crypto.Identity, bool, error) {
	if err := s.requireVault(); err != nil {
		return nil, false, err
	}
	defer s.checkPerms()
	s.checkPasswordChange()
	upgraded, err := s.migrate()
	if err != nil {
		return nil, false, err
	}
	id, err := s.sessionIdentity()
	switch {
	case err == nil:
		return id, upgraded, nil
	case !errors.Is(err, session.ErrNoSession) && !errors.Is(err, session.ErrExpired) && !errors.Is(err, session.ErrUnprotected):
		return nil, false, err
	}
	if id, err = ask(); err != nil {
		return nil, false, err
	}
	s.ackPasswordChange()
	return id, upgraded, s.sessions().Save(id.String())
}

func (s *Service) sessionIdentity() (*crypto.Identity, error) {
	str, err := s.sessions().Load()
	if errors.Is(err, session.ErrUnprotected) {
		s.warn(fmt.Sprintf("dropped unprotected session %s, unlock again", s.Paths.Session()))
	}
	if err != nil {
		return nil, err
	}
	return crypto.ParseIdentity(str)
}

func asTyped(secret string) string { return secret }

func (s *Service) askUnwrap(name, prompt string, canon func(string) string) (*crypto.Identity, error) {
	return Retry(s.Prompter, func() (*crypto.Identity, error) {
		secret, err := s.Prompter.Password(prompt)
		if err != nil {
			return nil, err
		}
		return s.unwrap(name, canon(secret))
	}, crypto.ErrWrongSecret)
}

func (s *Service) unwrap(name, secret string) (*crypto.Identity, error) {
	meta, err := format.LoadMeta(s.vaultFile(vaultfiles.MetaFile))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, ErrNoVault
	}
	if err != nil {
		return nil, err
	}
	data, err := fsutil.ReadFileLimit(s.vaultFile(name), maxIdentitySize)
	if err != nil {
		return nil, err
	}
	id, err := crypto.UnwrapIdentity(data, secret)
	if err != nil {
		return nil, err
	}
	if id.Recipient().String() != meta.Recipient {
		return nil, fmt.Errorf("%s: %w", name, vault.ErrRecipientMismatch)
	}
	return id, nil
}
