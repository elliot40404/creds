package app

import (
	"archive/tar"
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"

	"github.com/elliot40404/creds/internal/fsutil"
	"github.com/elliot40404/creds/internal/vaultfiles"
)

var ErrNotRegular = errors.New("not a regular file")

func (s *Service) ExportEncrypted(path string, overwrite bool) (int, error) {
	if err := s.requireVault(); err != nil {
		return 0, err
	}
	if err := checkTarget(path, overwrite); err != nil {
		return 0, err
	}
	root, err := os.OpenRoot(s.Paths.Vault())
	if err != nil {
		return 0, err
	}
	defer func() { _ = root.Close() }()
	names, err := backupNames(root)
	if err != nil {
		return 0, err
	}
	data, err := tarFiles(root, names)
	if err != nil {
		return 0, err
	}
	return len(names), fsutil.WriteFileAtomic(path, data)
}

func backupNames(root *os.Root) ([]string, error) {
	var names []string
	for _, n := range vaultfiles.Fixed() {
		_, err := root.Lstat(n)
		switch {
		case err == nil:
			names = append(names, n)
		case !errors.Is(err, fs.ErrNotExist):
			return nil, err
		}
	}
	buckets, err := fs.ReadDir(root.FS(), vaultfiles.EntriesDir)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}
	for _, b := range buckets {
		if name := vaultfiles.EntriesDir + "/" + b.Name(); vaultfiles.Allowed(name) {
			names = append(names, name)
		}
	}
	return names, nil
}

func tarFiles(root *os.Root, names []string) ([]byte, error) {
	var b bytes.Buffer
	w := tar.NewWriter(&b)
	for _, n := range names {
		if err := tarFile(w, root, n); err != nil {
			return nil, err
		}
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

func tarFile(w *tar.Writer, root *os.Root, name string) error {
	info, err := root.Lstat(name)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("%w: %s", ErrNotRegular, name)
	}
	data, err := root.ReadFile(name)
	if err != nil {
		return err
	}
	hdr := &tar.Header{Name: name, Mode: int64(fsutil.FilePerm), Size: int64(len(data)), ModTime: info.ModTime(), Format: tar.FormatPAX}
	if err := w.WriteHeader(hdr); err != nil {
		return err
	}
	_, err = w.Write(data)
	return err
}
