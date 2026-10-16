package app

import (
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"

	"github.com/elliot40404/creds/internal/sshdir"
	"github.com/elliot40404/creds/internal/vault"
)

type SSHAddHostResult struct {
	Dir     string
	Rel     string
	Alias   string
	Include string
}

func (s *Service) SSHAddHost(h SSHHost) (SSHAddHostResult, error) {
	if err := h.check(); err != nil {
		return SSHAddHostResult{}, err
	}
	dir, home, err := s.sshDirs()
	if err != nil {
		return SSHAddHostResult{}, err
	}
	res, err := scanSSH(dir)
	if err != nil {
		return SSHAddHostResult{}, err
	}
	if err := checkSSHKey(res, h.Key); err != nil {
		return SSHAddHostResult{}, err
	}
	f := sshdir.File{Rel: h.Rel(), Kind: sshdir.KindConfig, Data: h.config()}
	if err := s.sshFree(dir, f.Rel); err != nil {
		return SSHAddHostResult{}, err
	}
	if err := s.addSSHFile(dir, f); err != nil {
		return SSHAddHostResult{}, err
	}
	out := SSHAddHostResult{Dir: dir, Rel: f.Rel, Alias: h.Alias()}
	if res.Files = append(res.Files, f); !sshdir.Reaches(res, dir, home, f.Rel) {
		out.Include = h.IncludeLine()
	}
	return out, nil
}

func checkSSHKey(res sshdir.Result, key string) error {
	if key == "" || slices.ContainsFunc(res.Keys(), func(f sshdir.File) bool { return f.Rel == key }) {
		return nil
	}
	return fmt.Errorf("%w: key %q is not a private key file in ~/.ssh", ErrSSHHost, key)
}

func (s *Service) sshFree(dir, rel string) error {
	_, err := os.Lstat(filepath.Join(dir, filepath.FromSlash(rel)))
	switch {
	case err == nil:
		return fmt.Errorf("%w: %s is in %s", ErrSSHExists, rel, dir)
	case !errors.Is(err, fs.ErrNotExist):
		return err
	}
	p, err := SSHVaultPath(rel)
	if err != nil {
		return err
	}
	v, err := s.open()
	if err != nil {
		return err
	}
	defer v.Close()
	_, err = v.Get(p)
	switch {
	case err == nil:
		return fmt.Errorf("%w: %s is in the vault", ErrSSHExists, p)
	case errors.Is(err, vault.ErrNotFound):
		return nil
	}
	return err
}

func (s *Service) addSSHFile(dir string, f sshdir.File) error {
	out := []SSHFileResult{{Rel: f.Rel, Change: SSHRemoteNew, Action: SSHCreated}}
	disk := SSHStatus{Dir: dir, Files: []SSHFileStatus{{Rel: f.Rel, Kind: f.Kind, Vault: &f}}}
	if err := oneSSHErr(writeSSH(disk, out), out); err != nil {
		return err
	}
	out[0].Change, out[0].Action = SSHLocalNew, SSHPushed
	err := oneSSHErr(s.pushSSH([]SSHFileStatus{{Rel: f.Rel, Kind: f.Kind, Disk: &f}}, out), out)
	if err != nil {
		return fmt.Errorf("%s is written in %s but not saved in the vault, run creds ssh push: %w", f.Rel, dir, err)
	}
	return s.saveSSHState(func(cur sshdir.State) sshdir.State {
		next := sshdir.State{Files: maps.Clone(cur.Files), Skipped: cur.Skipped}
		if next.Files == nil {
			next.Files = map[string]sshdir.FileState{}
		}
		next.Files[f.Rel] = sshFileState(dir, f)
		return next
	})
}

func oneSSHErr(err error, out []SSHFileResult) error {
	if err != nil {
		return err
	}
	return out[0].Err
}
