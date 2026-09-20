//go:build !unix

package fsutil

func syncDir(string) error {
	return nil
}
