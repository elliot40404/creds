package app

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/elliot40404/creds/internal/sshdir"
)

type sshAdd struct {
	file  sshdir.File
	disk  bool
	vault bool
}

func (a sshAdd) action() SSHAction {
	switch {
	case a.disk:
		return SSHCreated
	case a.vault:
		return SSHPushed
	}
	return SSHNoChange
}

func (x SSH) addFiles(dir string, adds []sshAdd) ([]SSHFileResult, error) {
	if err := writeSSHAdds(dir, adds); err != nil {
		return nil, err
	}
	if err := x.pushAdds(adds); err != nil {
		if w := sshWritten(adds); w != "" {
			return nil, fmt.Errorf("%s is written in %s but not saved in the vault, run creds ssh push: %w", w, dir, err)
		}
		return nil, err
	}
	out := make([]SSHFileResult, len(adds))
	for i, a := range adds {
		out[i] = SSHFileResult{Rel: a.file.Rel, Action: a.action()}
	}
	return out, x.saveState(func(cur sshdir.State) sshdir.State { return addedSSHState(cur, dir, adds) })
}

func writeSSHAdds(dir string, adds []sshAdd) error {
	st := SSHStatus{Dir: dir, Files: make([]SSHFileStatus, len(adds))}
	out := make([]SSHFileResult, len(adds))
	for i := range adds {
		f := &adds[i].file
		st.Files[i] = SSHFileStatus{Rel: f.Rel, Kind: f.Kind, Vault: f}
		out[i] = sshAddStep(f.Rel, adds[i].disk, SSHCreated)
	}
	return firstSSHErr(writeSSH(st, out), out)
}

func (x SSH) pushAdds(adds []sshAdd) error {
	files := make([]SSHFileStatus, len(adds))
	out := make([]SSHFileResult, len(adds))
	for i := range adds {
		f := &adds[i].file
		files[i] = SSHFileStatus{Rel: f.Rel, Kind: f.Kind, Disk: f}
		out[i] = sshAddStep(f.Rel, adds[i].vault, SSHPushed)
	}
	return firstSSHErr(x.push(files, out), out)
}

func sshAddStep(rel string, do bool, a SSHAction) SSHFileResult {
	if !do {
		a = SSHNoChange
	}
	return SSHFileResult{Rel: rel, Action: a}
}

func firstSSHErr(err error, out []SSHFileResult) error {
	if err != nil {
		return err
	}
	for _, f := range out {
		if f.Err != nil {
			return f.Err
		}
	}
	return nil
}

func sshWritten(adds []sshAdd) string {
	var rels []string
	for _, a := range adds {
		if a.disk {
			rels = append(rels, a.file.Rel)
		}
	}
	return strings.Join(rels, ", ")
}

func addedSSHState(cur sshdir.State, dir string, adds []sshAdd) sshdir.State {
	next := sshdir.State{Files: maps.Clone(cur.Files), Skipped: slices.Clone(cur.Skipped)}
	if next.Files == nil {
		next.Files = map[string]sshdir.FileState{}
	}
	for _, a := range adds {
		next.Files[a.file.Rel] = sshFileState(dir, a.file)
		next.Skipped = slices.DeleteFunc(next.Skipped, func(rel string) bool { return rel == a.file.Rel })
	}
	return next
}
