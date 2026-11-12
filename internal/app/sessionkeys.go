package app

import (
	"errors"
	"sync"

	"github.com/elliot40404/creds/internal/keystore"
	"github.com/elliot40404/creds/internal/session"
)

const plainSessionWarning = "no OS key store here, the session is saved as a plain file, run creds lock when done"

type sessionKeys struct {
	once  sync.Once
	store keystore.Store
}

type sealOnce struct {
	keys keystore.Store
	mu   sync.Mutex
	err  error
}

func (s *Service) sessions() *session.Store {
	c := s.CurrentConfig()
	return &session.Store{
		Path:  s.Paths.Session(),
		Idle:  c.Session.Idle,
		Hard:  c.Session.Hard,
		Now:   s.Now,
		Keys:  s.sessionKeys(),
		Plain: !c.Session.ProtectOn(),
	}
}

func (s *Service) sessionKeys() keystore.Store {
	s.keys.once.Do(func() {
		open := s.Keystore
		if open == nil {
			open = keystore.Default
		}
		if store, err := open(); err == nil && store != nil {
			s.keys.store = &sealOnce{keys: store}
		}
	})
	return s.keys.store
}

func (s *Service) saveSession(id string) error {
	store := s.sessions()
	err := store.Save(id)
	if errors.Is(err, session.ErrPlainFallback) || (err == nil && store.Keys == nil && !store.Plain) {
		s.warn(plainSessionWarning)
		return nil
	}
	return err
}

func (k *sealOnce) Seal(account string, plain []byte) ([]byte, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.err != nil {
		return nil, k.err
	}
	sealed, err := k.keys.Seal(account, plain)
	k.err = err
	return sealed, err
}

func (k *sealOnce) Method() string { return k.keys.Method() }

func (k *sealOnce) Open(account string, sealed []byte) ([]byte, error) {
	return k.keys.Open(account, sealed)
}

func (k *sealOnce) Forget(account string) error { return k.keys.Forget(account) }
