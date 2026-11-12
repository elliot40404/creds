package app

import (
	"maps"
	"os"
	"path/filepath"
	"slices"

	"github.com/elliot40404/creds/internal/sshdir"
)

func (x SSH) saveState(next func(sshdir.State) sshdir.State) error {
	known := false
	_, err := sshdir.UpdateState(x.s.Paths.SSHState(), func(cur *sshdir.State) (bool, error) {
		n := next(*cur)
		known = len(n.Files) > 0
		if sameSSHState(*cur, n) {
			return false, nil
		}
		*cur = n
		return true, nil
	})
	if err == nil && known {
		x.turnOnAuto()
	}
	return err
}

func nextSSHState(cur sshdir.State, st SSHStatus, out []SSHFileResult) sshdir.State {
	next := sshdir.State{Files: map[string]sshdir.FileState{}}
	for i, f := range st.Files {
		if out[i].Action == SSHSkippedKey {
			next.Skipped = append(next.Skipped, f.Rel)
		}
		if fs, ok := sshBase(cur, st.Dir, f, out[i].Action); ok {
			next.Files[f.Rel] = fs
		}
	}
	return next
}

func sshBase(cur sshdir.State, dir string, f SSHFileStatus, a SSHAction) (sshdir.FileState, bool) {
	switch a {
	case SSHPushed, SSHNoChange:
		return sshFileState(dir, *f.Disk), true
	case SSHCreated, SSHUpdated:
		return sshFileState(dir, *f.Vault), true
	case SSHDeleted:
		return sshdir.FileState{}, false
	}
	fs, ok := cur.Files[f.Rel]
	return fs, ok
}

func sshFileState(dir string, f sshdir.File) sshdir.FileState {
	fs := sshdir.FileState{Sum: f.Sum()}
	info, err := os.Lstat(filepath.Join(dir, filepath.FromSlash(f.Rel)))
	if err == nil && info.Mode().IsRegular() && info.Size() == int64(len(f.Data)) {
		fs.Size, fs.ModTime = info.Size(), info.ModTime().UTC()
	}
	return fs
}

func sameSSHState(a, b sshdir.State) bool {
	return slices.Equal(a.Skipped, b.Skipped) && maps.EqualFunc(a.Files, b.Files, func(x, y sshdir.FileState) bool {
		return x.Sum == y.Sum && x.Size == y.Size && x.ModTime.Equal(y.ModTime)
	})
}
