package device

import (
	"encoding/json/v2"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/elliot40404/creds/internal/fsutil"
)

const (
	maxDeviceSize = 1 << 16
	lockSuffix    = ".lock"
)

var (
	ErrNotTrusted   = errors.New("device: this device is not trusted")
	ErrStale        = errors.New("device: the master password is needed again, device.max_age passed")
	ErrVaultChanged = errors.New("device: the vault key changed, device trust was removed")
	ErrUnprotected  = errors.New("device: trust file was readable by others and was removed")
	ErrCorrupt      = errors.New("device: trust file was corrupt and was removed")
	ErrMismatch     = errors.New("device: plugin identity does not open what its recipient sealed")
	ErrNoPlugin     = errors.New("device: age plugin not found on PATH")
)

type Store struct {
	Path   string
	MaxAge time.Duration
	Now    func() time.Time
}

type record struct {
	Plugin    string    `json:"plugin"`
	Identity  string    `json:"identity"`
	Recipient string    `json:"recipient"`
	Vault     string    `json:"vault"`
	Trusted   time.Time `json:"trusted"`
	Confirmed time.Time `json:"confirmed"`
	Sealed    []byte    `json:"sealed"`
}

type Status struct {
	Trusted   bool
	Plugin    string
	Since     time.Time
	Confirmed time.Time
	Expires   time.Time
	Stale     bool
}

func (s *Store) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

func (s *Store) Status() (Status, error) {
	r, err := s.load()
	if errors.Is(err, ErrNotTrusted) {
		return Status{}, nil
	}
	if err != nil {
		return Status{}, err
	}
	st := Status{Trusted: true, Plugin: r.Plugin, Since: r.Trusted, Confirmed: r.Confirmed, Stale: s.stale(r)}
	if s.MaxAge > 0 {
		st.Expires = r.Confirmed.Add(s.MaxAge)
	}
	return st, nil
}

func (s *Store) Confirm() error {
	return s.locked(func() error {
		r, err := s.load()
		if errors.Is(err, ErrNotTrusted) {
			return nil
		}
		if err != nil {
			return err
		}
		r.Confirmed = s.now()
		return fsutil.WriteJSON(s.Path, r)
	})
}

func (s *Store) Untrust() error {
	return s.locked(func() error {
		_, err := os.Lstat(s.Path)
		if errors.Is(err, fs.ErrNotExist) {
			return ErrNotTrusted
		}
		if err != nil {
			return err
		}
		return s.remove()
	})
}

func (s *Store) stale(r record) bool {
	return s.MaxAge > 0 && s.now().Sub(r.Confirmed) >= s.MaxAge
}

func (s *Store) save(r record) error {
	if err := fsutil.EnsureDir(filepath.Dir(s.Path)); err != nil {
		return err
	}
	return s.locked(func() error { return fsutil.WriteJSON(s.Path, r) })
}

func (s *Store) locked(fn func() error) error {
	return fsutil.WithFileLock(s.Path+lockSuffix, fn)
}

func (s *Store) remove() error {
	err := os.Remove(s.Path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	return err
}

func (s *Store) discard(reason error) error {
	if err := s.remove(); err != nil {
		return err
	}
	return reason
}

func (s *Store) load() (record, error) {
	r, err := s.read()
	if errors.Is(err, ErrCorrupt) || errors.Is(err, ErrUnprotected) {
		return r, s.discard(err)
	}
	return r, err
}

func (s *Store) read() (record, error) {
	var r record
	msg, err := fsutil.CheckPerms(s.Path)
	if errors.Is(err, fs.ErrNotExist) {
		return r, ErrNotTrusted
	}
	if err != nil || msg != "" {
		return r, ErrUnprotected
	}
	data, err := fsutil.ReadFileLimit(s.Path, maxDeviceSize)
	if errors.Is(err, fs.ErrNotExist) {
		return r, ErrNotTrusted
	}
	if err != nil {
		return r, ErrCorrupt
	}
	if err := json.Unmarshal(data, &r, json.RejectUnknownMembers(true)); err != nil || !valid(r) {
		return r, ErrCorrupt
	}
	return r, nil
}

func valid(r record) bool {
	return r.Plugin != "" && r.Identity != "" && r.Vault != "" && len(r.Sealed) > 0 &&
		!r.Trusted.IsZero() && !r.Confirmed.Before(r.Trusted)
}
