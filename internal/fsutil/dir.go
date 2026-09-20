package fsutil

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

const DirPerm os.FileMode = 0o700

func EnsureDir(path string) error {
	top, err := missingTop(path)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(path, DirPerm); err != nil {
		return err
	}
	if top == "" {
		top = path
	}
	return secureDir(top)
}

func missingTop(path string) (string, error) {
	top := ""
	for p := filepath.Clean(path); ; p = filepath.Dir(p) {
		_, err := os.Lstat(p)
		if err == nil {
			return top, nil
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return "", err
		}
		top = p
		if filepath.Dir(p) == p {
			return top, nil
		}
	}
}

func CheckAbsent(path string) error {
	_, err := os.Lstat(path)
	switch {
	case err == nil:
		return fs.ErrExist
	case errors.Is(err, fs.ErrNotExist):
		return nil
	}
	return err
}
