package gitsync

import (
	"crypto/sha256"
	"errors"
	"path/filepath"
	"time"

	"github.com/elliot40404/creds/internal/fsutil"
	"github.com/elliot40404/creds/internal/vaultfiles"
)

const maxPasswordFile = 1 << 20

func (r *Repo) passwordSum() [sha256.Size]byte {
	data, err := fsutil.ReadFileLimit(filepath.Join(r.Dir, vaultfiles.PasswordFile), maxPasswordFile)
	if err != nil {
		return [sha256.Size]byte{}
	}
	return sha256.Sum256(data)
}

func AckPasswordChange(statePath, lockPath string, now time.Time) (err error) {
	st, err := LoadState(statePath)
	if err != nil || st.PasswordChanged.IsZero() {
		return err
	}
	lock, err := AcquireLock(lockPath, now)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, lock.Release()) }()
	return ClearPasswordChange(statePath)
}

func ClearPasswordChange(statePath string) error {
	_, err := UpdateState(statePath, func(st *State) (bool, error) {
		if st.PasswordChanged.IsZero() {
			return false, nil
		}
		st.PasswordChanged = time.Time{}
		return true, nil
	})
	return err
}
