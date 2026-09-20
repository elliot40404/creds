//go:build unix

package fsutil

import (
	"errors"
	"os"
	"syscall"
)

var syncDirFile = (*os.File).Sync

func syncDir(dir string) error {
	d, err := os.Open(dir)
	if err != nil {
		return err
	}
	err = syncDirFile(d)
	if errors.Is(err, syscall.EINVAL) || errors.Is(err, syscall.ENOTSUP) {
		err = nil
	}
	return errors.Join(err, d.Close())
}
