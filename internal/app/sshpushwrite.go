package app

import (
	"errors"
	"fmt"
	"slices"

	"github.com/elliot40404/creds/internal/sshdir"
	"github.com/elliot40404/creds/internal/vault"
)

func (s *Service) pushSSH(files []SSHFileStatus, out []SSHPushFile) error {
	if !slices.ContainsFunc(out, func(f SSHPushFile) bool { return f.Action == SSHPushed }) {
		return nil
	}
	errs := make([]error, len(out))
	_, err := s.write(func(v *vault.Vault) (vault.Entry, error) {
		for i := range out {
			errs[i] = nil
			if out[i].Action == SSHPushed {
				errs[i] = putSSH(v, files[i])
			}
		}
		return vault.Entry{}, nil
	})
	if err != nil {
		return err
	}
	for i, err := range errs {
		if err != nil {
			out[i].Action, out[i].Err = SSHFailed, err
		}
	}
	return nil
}

func putSSH(v *vault.Vault, f SSHFileStatus) error {
	e, err := SSHEntry(*f.Disk)
	if err != nil {
		return err
	}
	old, err := v.Get(e.Path)
	switch {
	case errors.Is(err, vault.ErrNotFound):
		err = sameSSH(nil, f.Vault, e.Path)
	case err != nil:
	case old.Type != vault.TypeSSHFile:
		err = fmt.Errorf("%w: %s is a %s entry", vault.ErrDuplicatePath, e.Path, old.Type)
	default:
		err = sameSSH(&old, f.Vault, e.Path)
		e.ID, e.Created = old.ID, old.Created
	}
	if err != nil {
		return err
	}
	_, err = v.Put(e)
	return err
}

func sameSSH(old *vault.Entry, want *sshdir.File, path string) error {
	var cur *sshdir.File
	if old != nil {
		f, err := SSHFile(*old)
		if err != nil {
			return err
		}
		cur = &f
	}
	if fileSum(cur) != fileSum(want) {
		return fmt.Errorf("%w: %s changed in the vault, run creds ssh push again", ErrStale, path)
	}
	return nil
}
