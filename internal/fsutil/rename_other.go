//go:build !windows

package fsutil

func transient(error) bool {
	return false
}
