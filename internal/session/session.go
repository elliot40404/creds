package session

import (
	"bytes"
	"encoding/json/v2"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/elliot40404/creds/internal/fsutil"
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
}

type record struct {
	Identity string    `json:"identity"`
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
	now := s.now()
	return s.write(record{Identity: identity, Created: now, LastUsed: now})
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
	r.LastUsed = now
	s.refresh(r, raw)
	return r.Identity, nil
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
	_ = s.locked(func() error {
		current, err := fsutil.ReadFileLimit(s.Path, maxSessionSize)
		if err != nil || !bytes.Equal(current, raw) {
			return err
		}
		beforeRefresh()
		return s.write(r)
	})
}

func (s *Store) Delete() error {
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
	if err := s.Delete(); err != nil {
		return err
	}
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
	return r.Identity != "" &&
		!r.Created.IsZero() &&
		!r.LastUsed.Before(r.Created) &&
		!r.LastUsed.After(now)
}
