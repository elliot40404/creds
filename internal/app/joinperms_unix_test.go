//go:build unix

package app

import (
	"io/fs"
	"path/filepath"
	"testing"
)

func TestJoinLeavesNoGroupReadableFile(t *testing.T) {
	t.Parallel()
	_, b, _, _ := joinedPair(t)
	err := filepath.WalkDir(b.Paths.Vault(), func(path string, d fs.DirEntry, err error) error {
		if err != nil || !d.Type().IsRegular() && !d.IsDir() {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if info.Mode().Perm()&0o077 != 0 {
			t.Errorf("%s is mode %04o", path, info.Mode().Perm())
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
