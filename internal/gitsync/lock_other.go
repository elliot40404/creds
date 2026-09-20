//go:build !unix && !windows

package gitsync

func processAlive(int) bool {
	return true
}
