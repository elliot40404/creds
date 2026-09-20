package gitsync

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/elliot40404/creds/internal/config"
	"github.com/elliot40404/creds/internal/fsutil"
	"github.com/elliot40404/creds/internal/vaultfiles"
)

const (
	Branch = "master"
	Remote = "origin"
)

type Repo struct {
	Dir string
	git *Git

	hook func(stage string)
}

func Open(dir string) *Repo {
	return &Repo{Dir: dir, git: NewGit(dir)}
}

func (r *Repo) Configure(s config.Sync) {
	r.git.sign, r.git.signKey, r.git.name, r.git.email = s.Sign, s.SignKey, s.Name, s.Email
}

func InitRepo(ctx context.Context, dir string) (*Repo, error) {
	if err := fsutil.EnsureDir(dir); err != nil {
		return nil, err
	}
	r := Open(dir)
	if _, err := r.git.Run(ctx, "init", "-q", "-b", Branch); err != nil {
		return nil, err
	}
	if err := r.configure(ctx); err != nil {
		return nil, err
	}
	path := filepath.Join(dir, vaultfiles.IgnoreFile)
	if err := fsutil.WriteFileAtomic(path, []byte(vaultfiles.IgnoreRules)); err != nil {
		return nil, err
	}
	return r, nil
}

func (r *Repo) configure(ctx context.Context) error {
	for _, kv := range [][2]string{{"user.name", r.git.userName()}, {"user.email", r.git.userEmail()}} {
		if _, err := r.git.Run(ctx, "config", kv[0], kv[1]); err != nil {
			return err
		}
	}
	return nil
}

func (r *Repo) ensureIgnore() error {
	path := filepath.Join(r.Dir, vaultfiles.IgnoreFile)
	if _, err := os.Lstat(path); !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return fsutil.WriteFileAtomic(path, []byte(vaultfiles.IgnoreRules))
}
