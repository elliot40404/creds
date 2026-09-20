package fsutil

import (
	"crypto/rand"
	"errors"
	"os"
	"path/filepath"
	"time"
)

const (
	FilePerm      os.FileMode = 0o600
	renameTries               = 10
	renameBackoff             = 20 * time.Millisecond
)

var renameIn = (*os.Root).Rename

func WithRoot(dir string, fn func(*os.Root) error) error {
	r, err := os.OpenRoot(dir)
	if err != nil {
		return err
	}
	return errors.Join(fn(r), r.Close())
}

func WriteFileAtomic(path string, data []byte) error {
	dir, base := filepath.Split(path)
	if dir == "" {
		dir = "."
	}
	return WithRoot(dir, func(r *os.Root) error { return WriteFileIn(r, base, data) })
}

func WriteFileIn(root *os.Root, name string, data []byte) (err error) {
	if err := root.MkdirAll(filepath.Dir(name), DirPerm); err != nil {
		return err
	}
	tmpName := filepath.Join(filepath.Dir(name), "."+filepath.Base(name)+".tmp-"+rand.Text())
	f, err := createIn(root, tmpName)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = f.Close()
			_ = root.Remove(tmpName)
		}
	}()
	if err = writeAndSync(f, data); err != nil {
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	if err = root.Chmod(tmpName, FilePerm); err != nil {
		return err
	}
	if err = rename(root, tmpName, name); err != nil {
		return err
	}
	return syncDir(filepath.Join(root.Name(), filepath.Dir(name)))
}

func writeAndSync(f *os.File, data []byte) error {
	if _, err := f.Write(data); err != nil {
		return err
	}
	return f.Sync()
}

func rename(root *os.Root, old, new string) error {
	var err error
	for range renameTries {
		if err = renameIn(root, old, new); err == nil || !transient(err) {
			return err
		}
		time.Sleep(renameBackoff)
	}
	return err
}
