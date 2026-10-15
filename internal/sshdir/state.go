package sshdir

import (
	"crypto/sha256"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"time"

	"github.com/elliot40404/creds/internal/fsutil"
)

const (
	maxStateSize    = 1 << 20
	stateLockSuffix = ".lock"
)

var ErrStateCorrupt = errors.New("ssh state file is corrupt")

type State struct {
	Files   map[string]FileState `json:"files,omitzero"`
	Skipped []string             `json:"skipped,omitzero"`
}

type FileState struct {
	Sum     [sha256.Size]byte `json:"sum"`
	Size    int64             `json:"size,omitzero"`
	ModTime time.Time         `json:"mtime,omitzero"`
}

func LoadState(path string) (State, error) {
	data, err := fsutil.ReadFileLimit(path, maxStateSize)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return State{}, nil
	case errors.Is(err, fsutil.ErrTooLarge):
		return State{}, corrupt(path, err)
	case err != nil:
		return State{}, err
	}
	var s State
	if err := json.Unmarshal(data, &s, json.RejectUnknownMembers(true)); err != nil {
		return State{}, corrupt(path, err)
	}
	if err := s.check(); err != nil {
		return State{}, corrupt(path, err)
	}
	return s, nil
}

func corrupt(path string, err error) error {
	return fmt.Errorf("%w: %s: %w", ErrStateCorrupt, path, err)
}

func SaveState(path string, s State) error {
	if err := s.check(); err != nil {
		return err
	}
	return fsutil.WriteJSON(path, s)
}

func UpdateState(path string, fn func(*State) (bool, error)) (State, error) {
	var st State
	err := fsutil.WithFileLock(path+stateLockSuffix, func() error {
		var err error
		if st, err = LoadState(path); err != nil {
			return err
		}
		changed, err := fn(&st)
		if err != nil || !changed {
			return err
		}
		return SaveState(path, st)
	})
	return st, err
}

func (s State) check() error {
	for rel := range s.Files {
		if err := checkRel(rel); err != nil {
			return err
		}
	}
	for _, rel := range s.Skipped {
		if err := checkRel(rel); err != nil {
			return err
		}
	}
	return nil
}

func checkRel(rel string) error {
	if !filepath.IsLocal(filepath.FromSlash(rel)) {
		return fmt.Errorf("%w: %q", ErrOutside, rel)
	}
	return nil
}
