package app

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/elliot40404/creds/internal/fsutil"
	"github.com/elliot40404/creds/internal/sshdir"
)

func writesDisk(f SSHFileResult) bool {
	return f.Action == SSHCreated || f.Action == SSHUpdated || f.Action == SSHDeleted
}

func writeSSH(st SSHStatus, out []SSHFileResult) error {
	if !slices.ContainsFunc(out, writesDisk) {
		return nil
	}
	if err := ensureSSHDir(st.Dir); err != nil {
		return err
	}
	return fsutil.WithRoot(st.Dir, func(r *os.Root) error {
		for i, f := range st.Files {
			if !writesDisk(out[i]) {
				continue
			}
			if err := applySSH(r, f, out[i].Action); err != nil {
				out[i].Action, out[i].Err = SSHFailed, err
			}
		}
		return nil
	})
}

func ensureSSHDir(dir string) error {
	_, err := os.Lstat(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return fsutil.EnsureDir(dir)
	}
	return err
}

func applySSH(r *os.Root, f SSHFileStatus, a SSHAction) error {
	if err := checkSSHTarget(r, f); err != nil {
		return err
	}
	name := filepath.FromSlash(f.Rel)
	if a == SSHDeleted {
		return r.Remove(name)
	}
	return fsutil.WriteFileIn(r, name, f.Vault.Data)
}

func checkSSHTarget(r *os.Root, f SSHFileStatus) error {
	cur, err := diskSum(r, f.Rel)
	if err != nil {
		return err
	}
	if cur != fileSum(f.Disk) {
		return fmt.Errorf("%w: %s changed on disk, run creds ssh pull again", ErrStale, f.Rel)
	}
	return nil
}

func diskSum(r *os.Root, rel string) (sshSum, error) {
	ok, err := plainParents(r, rel)
	if err != nil || !ok {
		return sshSum{}, err
	}
	data, err := fsutil.ReadRegular(r, filepath.FromSlash(rel), sshdir.MaxFileSize)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return sshSum{}, nil
	case err != nil:
		return sshSum{}, fmt.Errorf("%w %q: %s", ErrSSHPath, rel, err.Error())
	}
	return sshSum{sum: sha256.Sum256(data), ok: true}, nil
}

func plainParents(r *os.Root, rel string) (bool, error) {
	parts := strings.Split(rel, "/")
	for i := 1; i < len(parts); i++ {
		p := strings.Join(parts[:i], "/")
		info, err := r.Lstat(filepath.FromSlash(p))
		switch {
		case errors.Is(err, fs.ErrNotExist):
			return false, nil
		case err != nil:
			return false, err
		case !info.IsDir():
			return false, fmt.Errorf("%w %q: %s is a link or a file, not a folder", ErrSSHPath, rel, p)
		}
	}
	return true, nil
}
