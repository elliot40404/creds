package format

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/elliot40404/creds/internal/fsutil"
	"github.com/elliot40404/creds/internal/vaultfiles"
)

const backupDir = ".migrate-backup"

var errBadBackup = errors.New("unexpected content in " + backupDir + ", check it and remove it by hand")

type txn struct {
	dir     string
	touched map[string]bool
	created []string
}

func newTx(dir string) *txn {
	return &txn{dir: dir, touched: map[string]bool{}}
}

func (t *txn) read(name string) ([]byte, error) {
	if err := checkName(name); err != nil {
		return nil, err
	}
	var data []byte
	err := fsutil.WithRoot(t.dir, func(r *os.Root) (err error) {
		data, err = fsutil.ReadRegular(r, name, vaultfiles.MaxFileSize)
		return err
	})
	return data, err
}

func (t *txn) write(name string, data []byte) error {
	if err := checkName(name); err != nil {
		return err
	}
	return fsutil.WithRoot(t.dir, func(r *os.Root) error {
		if !t.touched[name] {
			if err := t.backup(r, name); err != nil {
				return err
			}
			t.touched[name] = true
		}
		return fsutil.WriteFileIn(r, name, data)
	})
}

func checkName(name string) error {
	if !filepath.IsLocal(name) || !vaultfiles.Allowed(filepath.ToSlash(name)) {
		return fmt.Errorf("invalid migration path %q", name)
	}
	return nil
}

func (t *txn) backup(r *os.Root, name string) error {
	data, err := fsutil.ReadRegular(r, name, vaultfiles.MaxFileSize)
	if errors.Is(err, fs.ErrNotExist) {
		t.created = append(t.created, name)
		return nil
	}
	if err != nil {
		return err
	}
	return fsutil.WriteFileIn(r, filepath.Join(backupDir, name), data)
}

func (t *txn) commit() error {
	return fsutil.WithRoot(t.dir, func(r *os.Root) error {
		return r.RemoveAll(backupDir)
	})
}

func (t *txn) rollback() error {
	err := fsutil.WithRoot(t.dir, func(r *os.Root) error {
		for _, name := range t.created {
			if err := r.Remove(name); err != nil && !errors.Is(err, fs.ErrNotExist) {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	return restore(t.dir)
}

func restore(dir string) error {
	return fsutil.WithRoot(dir, func(r *os.Root) error {
		info, err := r.Lstat(backupDir)
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		if err != nil {
			return err
		}
		if !info.IsDir() {
			return errBadBackup
		}
		files, err := backupFiles(r)
		if err != nil {
			return err
		}
		for name, data := range files {
			if err := fsutil.WriteFileIn(r, filepath.FromSlash(name), data); err != nil {
				return err
			}
		}
		return r.RemoveAll(backupDir)
	})
}

func backupFiles(r *os.Root) (map[string][]byte, error) {
	files := map[string][]byte{}
	err := fs.WalkDir(r.FS(), backupDir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || p == backupDir {
			return err
		}
		name := strings.TrimPrefix(p, backupDir+"/")
		if d.IsDir() {
			if name != vaultfiles.EntriesDir {
				return fmt.Errorf("%w: %s", errBadBackup, name)
			}
			return nil
		}
		if !d.Type().IsRegular() || !vaultfiles.Allowed(name) {
			return fmt.Errorf("%w: %s", errBadBackup, name)
		}
		data, err := fsutil.ReadRegular(r, path.Join(backupDir, name), vaultfiles.MaxFileSize)
		files[name] = data
		return err
	})
	return files, err
}
