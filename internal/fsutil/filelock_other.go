//go:build !unix && !windows

package fsutil

import "os"

func tryLockFile(*os.File) error {
	return nil
}

func unlockFile(*os.File) error {
	return nil
}
