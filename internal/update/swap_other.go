//go:build !windows

package update

import "github.com/elliot40404/creds/internal/fsutil"

func swap(bin, exe string) error {
	return fsutil.Rename(bin, exe)
}

func Cleanup() {}
