package update

import (
	"errors"
	"io/fs"
	"os"

	"github.com/elliot40404/creds/internal/fsutil"
)

func swap(bin, exe string) error {
	old := exe + ".old"
	if err := os.Remove(old); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	if err := fsutil.Rename(exe, old); err != nil {
		return err
	}
	if err := fsutil.Rename(bin, exe); err != nil {
		return errors.Join(err, fsutil.Rename(old, exe))
	}
	return nil
}

func Cleanup() {
	if exe, err := Exe(); err == nil {
		removeOld(exe)
	}
}

func removeOld(exe string) {
	_ = os.Remove(exe + ".old")
}
