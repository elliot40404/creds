package session

import (
	"bytes"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/elliot40404/creds/internal/fsutil"
	"github.com/elliot40404/creds/internal/keystore"
)

const (
	maxSessionSize = 1 << 16
	lockSuffix     = ".lock"
)

var beforeRefresh = func() {}

var (
	ErrNoSession   = errors.New("session: no session")
	ErrExpired     = errors.New("session: expired")
	ErrUnprotected = errors.New("session: file was readable by others and was dropped")
	errCorrupt     = errors.New("session: corrupt")
)

type Store struct {
	Path string
	Idle time.Duration
	Hard time.Duration
	Now  func() time.Time
	Keys keystore.Store
}

type record struct {
	Version  int       `json:"version,omitzero"`
	Method   string    `json:"method,omitzero"`
	Identity string    `json:"identity,omitzero"`
	Sealed   []byte    `json:"sealed,omitzero"`
	Created  time.Time `json:"created"`
	LastUsed time.Time `json:"last_used"`
}

func (s *Store) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

func (s *Store) Save(identity string) error {
	r, sealErr := s.protect(identity)
	r.Created = s.now()
	r.LastUsed = r.Created
	if err := s.write(r); err != nil {
		return err
	}
	return sealErr
}

func (s *Store) write(r record) error {
	if err := fsutil.EnsureDir(filepath.Dir(s.Path)); err != nil {
		return err
	}
	return fsutil.WriteJSON(s.Path, r)
}

func (s *Store) Load() (string, error) {
	now := s.now()
	r, raw, err := s.live(now)
	if err != nil {
		return "", err
	}
	id, err := s.reveal(r)
	if err != nil {
		return "", s.discard(fmt.Errorf("%w: %w", ErrNoSession, err))
	}
	r.LastUsed = now
	if s.upgradable(r) {
		s.upgrade(r, id, raw)
	} else {
		s.refresh(r, raw)
	}
	return id, nil
}

func (s *Store) Left() (time.Duration, error) {
	now := s.now()
	r, _, err := s.live(now)
	if err != nil {
		return 0, err
	}
	idle := s.Idle - now.Sub(r.LastUsed)
	hard := s.Hard - now.Sub(r.Created)
	return min(idle, hard), nil
}

func (s *Store) live(now time.Time) (record, []byte, error) {
	r, raw, err := s.read(now)
	switch {
	case errors.Is(err, errCorrupt):
		return r, nil, s.discard(ErrNoSession)
	case errors.Is(err, ErrUnprotected):
		return r, nil, s.discard(ErrUnprotected)
	case err != nil:
		return r, nil, err
	case s.expired(r, now):
		return r, nil, s.discard(ErrExpired)
	}
	return r, raw, nil
}

func (s *Store) refresh(r record, raw []byte) {
	s.swap(raw, func() record { return r })
}

func (s *Store) upgrade(r record, id string, raw []byte) {
	s.swap(raw, func() record {
		next, err := s.protect(id)
		if err != nil {
			return r
		}
		next.Created, next.LastUsed = r.Created, r.LastUsed
		return next
	})
}

func (s *Store) swap(raw []byte, next func() record) {
	_ = s.locked(func() error {
		current, err := fsutil.ReadFileLimit(s.Path, maxSessionSize)
		if err != nil || !bytes.Equal(current, raw) {
			return err
		}
		beforeRefresh()
		return s.write(next())
	})
}

func (s *Store) Delete() error {
	if err := s.drop(); err != nil {
		return err
	}
	return s.forget()
}

func (s *Store) drop() error {
	err := s.locked(s.remove)
	if errors.Is(err, fsutil.ErrBusy) || errors.Is(err, fs.ErrNotExist) {
		return s.remove()
	}
	return err
}

func (s *Store) remove() error {
	err := os.Remove(s.Path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	return err
}

func (s *Store) locked(fn func() error) error {
	return fsutil.WithFileLock(s.Path+lockSuffix, fn)
}

func (s *Store) expired(r record, now time.Time) bool {
	return now.Sub(r.LastUsed) >= s.Idle || now.Sub(r.Created) >= s.Hard
}

func (s *Store) discard(reason error) error {
	if err := s.drop(); err != nil {
		return err
	}
	_ = s.forget()
	return reason
}

func (s *Store) read(now time.Time) (record, []byte, error) {
	var r record
	msg, err := fsutil.CheckPerms(s.Path)
	if errors.Is(err, fs.ErrNotExist) {
		return r, nil, ErrNoSession
	}
	if err != nil || msg != "" {
		return r, nil, ErrUnprotected
	}
	data, err := fsutil.ReadFileLimit(s.Path, maxSessionSize)
	if errors.Is(err, fs.ErrNotExist) {
		return r, nil, ErrNoSession
	}
	if err != nil {
		return r, nil, errCorrupt
	}
	if err := json.Unmarshal(data, &r, json.RejectUnknownMembers(true)); err != nil {
		return r, nil, errCorrupt
	}
	if !valid(r, now) {
		return r, nil, errCorrupt
	}
	return r, data, nil
}

func valid(r record, now time.Time) bool {
	return validSecret(r) &&
		!r.Created.IsZero() &&
		!r.LastUsed.Before(r.Created) &&
		!r.LastUsed.After(now)
}
