//go:build !windows

package fsutil

import "os"

func secureDir(path string) error {
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm()&^DirPerm == 0 {
		return err
	}
	return os.Chmod(path, DirPerm)
}
