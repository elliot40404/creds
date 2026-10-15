package app

import (
	"maps"
	"os"
	"path/filepath"
	"slices"

	"github.com/elliot40404/creds/internal/sshdir"
)

func (s *Service) saveSSHState(st SSHStatus, out []SSHPushFile) error {
	_, err := sshdir.UpdateState(s.Paths.SSHState(), func(cur *sshdir.State) (bool, error) {
		next := nextSSHState(*cur, st, out)
		if sameSSHState(*cur, next) {
			return false, nil
		}
		*cur = next
		return true, nil
	})
	return err
}

func nextSSHState(cur sshdir.State, st SSHStatus, out []SSHPushFile) sshdir.State {
	next := sshdir.State{Files: map[string]sshdir.FileState{}}
	for i, f := range st.Files {
		switch out[i].Action {
		case SSHPushed, SSHNoChange:
			next.Files[f.Rel] = sshFileState(st.Dir, *f.Disk)
			continue
		case SSHSkippedKey:
			next.Skipped = append(next.Skipped, f.Rel)
		}
		if base, ok := cur.Files[f.Rel]; ok {
			next.Files[f.Rel] = base
		}
	}
	return next
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
