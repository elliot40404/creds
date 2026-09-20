package gitsync

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/elliot40404/creds/internal/crypto"
	"github.com/elliot40404/creds/internal/vaultfiles"
)

const peekSize = 64

var (
	ErrNotAllowed = errors.New("gitsync: path not allowed")
	ErrPlaintext  = errors.New("gitsync: file is not age encrypted")
)

type change struct {
	status string
	path   string
}

func encrypted(path string) bool {
	return strings.HasSuffix(path, ".age") || strings.HasSuffix(path, ".enc")
}

func checkStaged(changes []change) error {
	for _, c := range changes {
		if c.status != "D" && !vaultfiles.Allowed(c.path) {
			return fmt.Errorf("%w: %s", ErrNotAllowed, c.path)
		}
	}
	return nil
}

func (r *Repo) checkWorktree(paths []string) error {
	root, err := os.OpenRoot(r.Dir)
	if err != nil {
		return err
	}
	defer func() { _ = root.Close() }()
	for _, p := range paths {
		if err := checkFile(root, p); err != nil {
			return err
		}
	}
	return nil
}

func checkFile(root *os.Root, path string) error {
	name := filepath.FromSlash(path)
	info, err := root.Lstat(name)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("%w: %s is not a regular file", ErrNotAllowed, path)
	}
	if !encrypted(path) {
		return nil
	}
	head, err := readHead(root, name)
	if err != nil {
		return err
	}
	if !crypto.IsAgeFile(head) {
		return fmt.Errorf("%w: %s", ErrPlaintext, path)
	}
	return nil
}

func readHead(root *os.Root, name string) ([]byte, error) {
	f, err := root.Open(name)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	head := make([]byte, peekSize)
	n, err := io.ReadFull(f, head)
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		err = nil
	}
	return head[:n], err
}
