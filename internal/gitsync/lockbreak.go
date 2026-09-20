package gitsync

import (
	"errors"

	"github.com/elliot40404/creds/internal/fsutil"
)

const breakSuffix = ".break"

func withBreak(path string, fn func() error) error {
	err := fsutil.WithFileLock(path+breakSuffix, fn)
	if errors.Is(err, fsutil.ErrBusy) {
		return errors.Join(ErrLocked, err)
	}
	return err
}
