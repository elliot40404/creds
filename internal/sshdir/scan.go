package sshdir

import (
	"crypto/sha256"
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/elliot40404/creds/internal/fsutil"
)

const MaxFileSize = 64 << 10

type Kind int

const (
	KindConfig Kind = iota + 1
	KindPrivateKey
	KindPublicKey
)

var ErrSymlink = errors.New("symlink skipped")

type File struct {
	Rel  string
	Kind Kind
	Data []byte
}

func (f File) Sum() [sha256.Size]byte {
	return sha256.Sum256(f.Data)
}

type Skip struct {
	Rel string
	Err error
}

type Result struct {
	Files   []File
	Skipped []Skip
}

func (r Result) Keys() []File {
	var keys []File
	for _, f := range r.Files {
		if f.Kind == KindPrivateKey {
			keys = append(keys, f)
		}
	}
	return keys
}

func Scan(dir string) (Result, error) {
	root, err := os.OpenRoot(dir)
	if err != nil {
		return Result{}, err
	}
	defer func() { _ = root.Close() }()
	s := scanner{root: root}
	if err := fs.WalkDir(root.FS(), ".", s.visit); err != nil {
		return Result{}, err
	}
	return s.res, nil
}

type scanner struct {
	root *os.Root
	res  Result
}

func (s *scanner) visit(rel string, d fs.DirEntry, err error) error {
	switch {
	case err != nil && rel == ".":
		return err
	case err != nil:
		s.skip(rel, err)
	case rel == "." || d.IsDir() || Ignored(d.Name()):
	case d.Type()&fs.ModeSymlink != 0:
		s.skip(rel, ErrSymlink)
	case d.Type().IsRegular():
		s.read(rel)
	}
	return nil
}

func (s *scanner) read(rel string) {
	data, err := fsutil.ReadRegular(s.root, filepath.FromSlash(rel), MaxFileSize)
	if err != nil {
		s.skip(rel, err)
		return
	}
	if kind := Classify(rel, data); kind != 0 {
		s.res.Files = append(s.res.Files, File{Rel: rel, Kind: kind, Data: data})
	}
}

func (s *scanner) skip(rel string, err error) {
	s.res.Skipped = append(s.res.Skipped, Skip{Rel: rel, Err: err})
}
