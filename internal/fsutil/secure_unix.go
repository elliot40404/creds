//go:build unix

package fsutil

import (
	"io/fs"
	"os"
)

func secureTree(dir string) error {
	root, err := os.OpenRoot(dir)
	if err != nil {
		return err
	}
	defer func() { _ = root.Close() }()
	return fs.WalkDir(root.FS(), ".", func(path string, d fs.DirEntry, err error) error {
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
		return root.Chmod(path, want)
	})
}
