package app

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"slices"

	"github.com/elliot40404/creds/internal/config"
	"github.com/elliot40404/creds/internal/fsutil"
	"github.com/elliot40404/creds/internal/vaultfiles"
)

func (s *Service) Warnings() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	w := append(config.Warnings(s.Config), s.warnings...)
	s.warnings = nil
	return w
}

func (s *Service) warn(msg string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !slices.Contains(s.warnings, msg) {
		s.warnings = append(s.warnings, msg)
	}
}

func (s *Service) checkPerms() {
	if err := secureHome(s.Paths.Home); err != nil {
		s.warn(fmt.Sprintf("could not protect %s: %v", s.Paths.Home, err))
	}
	paths := []string{s.Paths.Home, s.Paths.Vault(), s.vaultFile(vaultfiles.PasswordFile), s.vaultFile(vaultfiles.RecoveryFile)}
	for _, p := range paths {
		msg, err := fsutil.CheckPerms(p)
		switch {
		case errors.Is(err, fs.ErrNotExist):
		case err != nil:
			s.warn(fmt.Sprintf("could not check access to %s: %v", p, err))
		case msg != "":
			s.warn(msg)
		}
	}
}

func secureHome(home string) error {
	if _, err := os.Stat(home); errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	return fsutil.EnsureDir(home)
}
