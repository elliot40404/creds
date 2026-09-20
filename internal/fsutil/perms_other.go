//go:build !unix && !windows

package fsutil

import "os"

func CheckPerms(path string) (string, error) {
	_, err := os.Stat(path)
	return "", err
}
