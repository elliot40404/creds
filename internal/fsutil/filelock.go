package fsutil

import (
	"errors"
	"os"
	"path/filepath"
	"time"
)

const (
	lockTries   = 100
	lockBackoff = 5 * time.Millisecond
)

var ErrBusy = errors.New("file lock is busy")

func WithFileLock(path string, fn func() error) (err error) {
	f, err := os.OpenFile(filepath.Clean(path), os.O_RDWR|os.O_CREATE, FilePerm)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, f.Close()) }()
	for range lockTries {
		if err = tryLockFile(f); err == nil {
			defer func() { err = errors.Join(err, unlockFile(f)) }()
			return fn()
		}
		time.Sleep(lockBackoff)
	}
	return errors.Join(ErrBusy, err)
}
