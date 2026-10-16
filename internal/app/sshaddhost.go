package app

import (
	"errors"
	"fmt"
	"io/fs"
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
	if _, err := s.addSSHFiles(dir, []sshAdd{{file: f, disk: true, vault: true}}); err != nil {
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

func (s *Service) sshFree(dir string, rels ...string) error {
	for _, rel := range rels {
		_, err := os.Lstat(filepath.Join(dir, filepath.FromSlash(rel)))
		switch {
		case err == nil:
			return fmt.Errorf("%w: %s is in %s", ErrSSHExists, rel, dir)
		case !errors.Is(err, fs.ErrNotExist):
			return err
		}
	}
	v, err := s.open()
	if err != nil {
		return err
	}
	defer v.Close()
	for _, rel := range rels {
		if err := sshVaultFree(v, rel); err != nil {
			return err
		}
	}
	return nil
}

func sshVaultFree(v *vault.Vault, rel string) error {
	p, err := SSHVaultPath(rel)
	if err != nil {
		return err
	}
	_, err = v.Get(p)
	switch {
	case err == nil:
		return fmt.Errorf("%w: %s is in the vault", ErrSSHExists, p)
	case errors.Is(err, vault.ErrNotFound):
		return nil
	}
	return err
}
