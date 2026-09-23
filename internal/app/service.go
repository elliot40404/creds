package app

import (
	"errors"
	"path/filepath"
	"sync"
	"time"

	"github.com/elliot40404/creds/internal/config"
	"github.com/elliot40404/creds/internal/crypto"
	"github.com/elliot40404/creds/internal/session"
)

var (
	ErrVaultExists      = errors.New("vault already exists")
	ErrNoVault          = errors.New("vault not initialized")
	ErrPasswordMismatch = errors.New("passwords do not match")
	ErrRecoveryMismatch = errors.New("recovery code does not match")
	ErrAborted          = errors.New("aborted")
	ErrWeakPassword     = errors.New("password is too weak")
)

type Service struct {
	Paths    config.Paths
	Config   config.Config
	Prompter Prompter
	Now      func() time.Time
	LogN     int
	Spawn    func() error
	Waiting  func()

	mu           sync.Mutex
	readChecked  bool
	strayChecked bool
	warnings     []string
}

func (s *Service) logN() int {
	if s.LogN > 0 {
		return s.LogN
	}
	return crypto.ScryptLogN
}

func (s *Service) CurrentConfig() config.Config {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.Config
}

func (s *Service) setConfig(c config.Config) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Config = c
}

func (s *Service) firstTime(done *bool) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if *done {
		return false
	}
	*done = true
	return true
}

func (s *Service) sessions() *session.Store {
	c := s.CurrentConfig()
	return &session.Store{
		Path: s.Paths.Session(),
		Idle: c.Session.Idle,
		Hard: c.Session.Hard,
		Now:  s.Now,
	}
}

func (s *Service) vaultFile(name string) string {
	return filepath.Join(s.Paths.Vault(), name)
}

func (s *Service) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}
