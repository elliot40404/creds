package app

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"slices"

	"github.com/elliot40404/creds/internal/fsutil"
	"github.com/elliot40404/creds/internal/sshdir"
	"github.com/elliot40404/creds/internal/vault"
)

type SSHAddKeyResult struct {
	Dir     string
	Rel     string
	Files   []SSHFileResult
	Unused  bool
	PubNote string
}

func (x SSH) AddKey(file, name string) (SSHAddKeyResult, error) {
	dir, home, err := x.dirs()
	if err != nil {
		return SSHAddKeyResult{}, err
	}
	res, err := scanSSH(dir)
	if err != nil {
		return SSHAddKeyResult{}, err
	}
	files, note, inside, err := sshKeyFiles(res, dir, file, name)
	if err != nil {
		return SSHAddKeyResult{}, err
	}
	adds, err := x.planAdd(dir, files, inside)
	if err != nil {
		return SSHAddKeyResult{}, err
	}
	out, err := x.addFiles(dir, adds)
	if err != nil {
		return SSHAddKeyResult{}, err
	}
	rel := files[0].Rel
	unused := sshdir.ReadUsage(withSSHFiles(res, files), dir, home).Unused
	return SSHAddKeyResult{
		Dir:     dir,
		Rel:     rel,
		Files:   out,
		Unused:  slices.ContainsFunc(unused, func(f sshdir.File) bool { return f.Rel == rel }),
		PubNote: note,
	}, nil
}

func sshKeyFiles(res sshdir.Result, dir, file, name string) ([]sshdir.File, string, bool, error) {
	abs, err := filepath.Abs(file)
	if err != nil {
		return nil, "", false, err
	}
	if rel, ok := sshInside(dir, abs); ok {
		files, note, err := insideSSHKey(res, rel, name)
		return files, note, true, err
	}
	key, err := outsideSSHKey(abs, name)
	if err != nil {
		return nil, "", false, err
	}
	data, err := fsutil.ReadFileLimit(abs+".pub", sshdir.MaxFileSize)
	files, note := withSSHPub(key, data, err)
	return files, note, false, nil
}

func (x SSH) planAdd(dir string, files []sshdir.File, inside bool) ([]sshAdd, error) {
	v, err := x.s.open()
	if err != nil {
		return nil, err
	}
	defer v.Close()
	adds := make([]sshAdd, len(files))
	for i, f := range files {
		inVault, err := sshVaultHas(v, f)
		if err != nil {
			return nil, err
		}
		onDisk := inside
		if !inside {
			if onDisk, err = sshDiskHas(dir, f); err != nil {
				return nil, err
			}
		}
		adds[i] = sshAdd{file: f, disk: !onDisk, vault: !inVault}
	}
	return adds, nil
}

func sshVaultHas(v *vault.Vault, f sshdir.File) (bool, error) {
	p, err := SSHVaultPath(f.Rel)
	if err != nil {
		return false, err
	}
	e, err := v.Get(p)
	switch {
	case errors.Is(err, vault.ErrNotFound):
		return false, nil
	case err != nil:
		return false, err
	}
	if cur, err := SSHFile(e); err == nil && cur.Sum() == f.Sum() {
		return true, nil
	}
	return false, fmt.Errorf("%w: %s differs in the vault, run creds ssh status", ErrSSHExists, p)
}

func sshDiskHas(dir string, f sshdir.File) (bool, error) {
	data, err := fsutil.ReadFileLimit(filepath.Join(dir, filepath.FromSlash(f.Rel)), sshdir.MaxFileSize)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return false, nil
	case err == nil && bytes.Equal(data, f.Data):
		return true, nil
	}
	return false, fmt.Errorf("%w: %s is in %s with other content", ErrSSHExists, f.Rel, dir)
}

func withSSHFiles(res sshdir.Result, files []sshdir.File) sshdir.Result {
	all := slices.Clone(res.Files)
	for _, f := range files {
		if !slices.ContainsFunc(all, func(g sshdir.File) bool { return g.Rel == f.Rel }) {
			all = append(all, f)
		}
	}
	res.Files = all
	return res
}
