package fsutil

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

var (
	errNotRegular = errors.New("not a regular file")

	ErrTooLarge = errors.New("too large")
)

func ReadFileLimit(path string, limit int64) ([]byte, error) {
	root, err := os.OpenRoot(filepath.Dir(path))
	if err != nil {
		return nil, err
	}
	defer func() { _ = root.Close() }()
	return ReadRegular(root, filepath.Base(path), limit)
}

func ReadRegular(root *os.Root, name string, limit int64) ([]byte, error) {
	info, err := root.Lstat(name)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("%s: %w", name, errNotRegular)
	}
	f, err := root.Open(name)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	data, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("%s: %w", name, ErrTooLarge)
	}
	return data, nil
}
