package session

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"path/filepath"
)

const (
	version     = 2
	methodPlain = "plain"
)

var (
	ErrPlainFallback = errors.New("session: saved without os key store protection")
	errNoKeys        = errors.New("session: sealed session but no key store")
	errMethod        = errors.New("session: sealed by another key store")
)

func (s *Store) protect(identity string) (record, error) {
	plain := record{Version: version, Method: methodPlain, Identity: identity}
	if s.Keys == nil || s.Plain {
		return plain, nil
	}
	secret := []byte(identity)
	defer clear(secret)
	sealed, err := s.Keys.Seal(s.account(), secret)
	if err != nil {
		return plain, fmt.Errorf("%w: %w", ErrPlainFallback, err)
	}
	return record{Version: version, Method: s.Keys.Method(), Sealed: sealed}, nil
}

func (s *Store) reveal(r record) (string, error) {
	switch {
	case !r.sealed():
		return r.Identity, nil
	case s.Keys == nil:
		return "", errNoKeys
	case r.Method != s.Keys.Method():
		return "", errMethod
	}
	secret, err := s.Keys.Open(s.account(), r.Sealed)
	if err != nil {
		return "", err
	}
	defer clear(secret)
	if len(secret) == 0 {
		return "", errCorrupt
	}
	return string(secret), nil
}

func (s *Store) upgradable(r record) bool {
	return s.Keys != nil && !s.Plain && !r.sealed()
}

func (s *Store) forget() error {
	if s.Keys == nil {
		return nil
	}
	return s.Keys.Forget(s.account())
}

func (s *Store) account() string {
	path, err := filepath.Abs(s.Path)
	if err != nil {
		path = filepath.Clean(s.Path)
	}
	sum := sha256.Sum256([]byte("creds session\x00" + path))
	return hex.EncodeToString(sum[:])
}

func (r record) method() string {
	if r.Method == "" {
		return methodPlain
	}
	return r.Method
}

func (r record) sealed() bool {
	return r.Version == version && r.Method != methodPlain
}

func validSecret(r record) bool {
	switch {
	case r.Version == 0:
		return r.Method == "" && len(r.Sealed) == 0 && r.Identity != ""
	case r.Version != version:
		return false
	case r.Method == methodPlain:
		return r.Identity != "" && len(r.Sealed) == 0
	}
	return r.Method != "" && r.Identity == "" && len(r.Sealed) > 0
}
