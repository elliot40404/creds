package app

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/elliot40404/creds/internal/sshdir"
	"github.com/elliot40404/creds/internal/vault"
)

type SSHFileStatus struct {
	Rel     string
	Kind    sshdir.Kind
	Change  SSHChange
	Disk    *sshdir.File
	Vault   *sshdir.File
	Unused  bool
	Skipped bool
}

type SSHStatus struct {
	Dir      string
	Files    []SSHFileStatus
	Skipped  []sshdir.Skip
	Missing  []string
	Warnings []error
	Invalid  []error
}

func (x SSH) Status() (SSHStatus, error) {
	dir, home, err := x.dirs()
	if err != nil {
		return SSHStatus{}, err
	}
	remote, invalid, err := x.vaultFiles()
	if err != nil {
		return SSHStatus{}, err
	}
	res, err := scanSSH(dir)
	if err != nil {
		return SSHStatus{}, err
	}
	st, err := sshdir.LoadState(x.s.Paths.SSHState())
	if err != nil {
		return SSHStatus{}, err
	}
	u := sshdir.ReadUsage(res, dir, home)
	return SSHStatus{
		Dir:      dir,
		Files:    sshFiles(res.Files, remote, st, u.Unused),
		Skipped:  res.Skipped,
		Missing:  u.Missing,
		Warnings: u.Warnings,
		Invalid:  invalid,
	}, nil
}

func (x SSH) dirs() (dir, home string, err error) {
	home = x.s.UserHome
	if home == "" {
		if home, err = os.UserHomeDir(); err != nil {
			return "", "", err
		}
	}
	return filepath.Join(home, ".ssh"), home, nil
}

func (x SSH) vaultFiles() ([]sshdir.File, []error, error) {
	v, err := x.s.open()
	if err != nil {
		return nil, nil, err
	}
	defer v.Close()
	var files []sshdir.File
	var invalid []error
	for _, e := range v.List() {
		if e.Type != vault.TypeSSHFile {
			continue
		}
		f, err := SSHFile(e)
		if err != nil {
			invalid = append(invalid, err)
			continue
		}
		files = append(files, f)
	}
	return files, invalid, nil
}

func scanSSH(dir string) (sshdir.Result, error) {
	res, err := sshdir.Scan(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return sshdir.Result{}, nil
	}
	return res, err
}

func sshFiles(disk, remote []sshdir.File, st sshdir.State, unused []sshdir.File) []SSHFileStatus {
	byRel := map[string]*SSHFileStatus{}
	at := func(f sshdir.File) *SSHFileStatus {
		if byRel[f.Rel] == nil {
			byRel[f.Rel] = &SSHFileStatus{Rel: f.Rel, Kind: f.Kind}
		}
		return byRel[f.Rel]
	}
	for i := range disk {
		at(disk[i]).Disk = &disk[i]
	}
	for i := range remote {
		at(remote[i]).Vault = &remote[i]
	}
	for _, f := range unused {
		at(f).Unused = true
	}
	out := make([]SSHFileStatus, 0, len(byRel))
	for _, f := range byRel {
		f.Change = sshCompare(fileSum(f.Disk), fileSum(f.Vault), baseSum(st, f.Rel))
		f.Skipped = slices.Contains(st.Skipped, f.Rel)
		out = append(out, *f)
	}
	slices.SortFunc(out, func(a, b SSHFileStatus) int { return strings.Compare(a.Rel, b.Rel) })
	return out
}
