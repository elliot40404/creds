//go:build unix

package fsutil

import (
	"io/fs"
	"os"
	"path/filepath"
)

func secureTree(dir string) error {
	return filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || !d.Type().IsRegular() && !d.IsDir() {
			return err
		}
		want := FilePerm
		if d.IsDir() {
			want = DirPerm
		}
		info, err := d.Info()
		if err != nil || info.Mode().Perm()&^want == 0 {
			return err
		}
		return os.Chmod(path, want)
	})
}
