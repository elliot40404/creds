package fsutil

import (
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	lockTries   = 400
	lockBackoff = 5 * time.Millisecond
)

var ErrBusy = errors.New("file lock is busy")

type Held struct {
	f    *os.File
	once sync.Once
	err  error
}

func TryHold(path string) (*Held, error) {
	f, err := openLockFile(path)
	if err != nil {
		return nil, err
	}
	if err := tryLockFile(f); err != nil {
		cerr := f.Close()
		if isBusy(err) {
			return nil, errors.Join(ErrBusy, err, cerr)
		}
		return nil, errors.Join(err, cerr)
	}
	return &Held{f: f}, nil
}

func openLockFile(path string) (*os.File, error) {
	root, err := os.OpenRoot(filepath.Dir(path))
	if err != nil {
		return nil, err
	}
	defer func() { _ = root.Close() }()
	return root.OpenFile(filepath.Base(path), os.O_RDWR|os.O_CREATE, FilePerm)
}

func (h *Held) Close() error {
	h.once.Do(func() {
		h.err = errors.Join(unlockFile(h.f), h.f.Close())
	})
	return h.err
}

func WithFileLock(path string, fn func() error) (err error) {
	h, err := holdWithRetry(path)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, h.Close()) }()
	return fn()
}

func holdWithRetry(path string) (*Held, error) {
	var err error
	for range lockTries {
		var h *Held
		if h, err = TryHold(path); !errors.Is(err, ErrBusy) {
			return h, err
		}
		time.Sleep(lockBackoff)
	}
	return nil, err
}
