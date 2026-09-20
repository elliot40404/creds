//go:build !windows

package fsutil

import "os"

func createIn(root *os.Root, name string) (*os.File, error) {
	return root.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, FilePerm)
}
