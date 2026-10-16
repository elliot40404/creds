package app

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"github.com/elliot40404/creds/internal/session"
	"github.com/elliot40404/creds/internal/sshdir"
)

func (s *Service) SSHAutoPush() string {
	dir, _, err := s.sshDirs()
	if err != nil {
		return ""
	}
	st, err := sshdir.LoadState(s.Paths.SSHState())
	if err != nil || !s.sshAutoOn(st) {
		return ""
	}
	dirty := sshStatDirty(dir, st)
	if len(dirty) == 0 {
		return ""
	}
	c := s.sessionOnlyService()
	res, err := c.SSHPush(SSHPushOptions{KnownOnly: true})
	for _, w := range c.takeWarnings() {
		s.warn(w)
	}
	switch {
	case errors.Is(err, session.ErrNoSession):
		return ""
	case err != nil:
		return "ssh: auto push failed, run creds ssh push"
	}
	return sshAutoHint(sshPushLeft(res.Files, dirty), sshPushWords)
}

func (s *Service) sshAutoOn(st sshdir.State) bool {
	return len(st.Files) > 0
}

func (s *Service) sessionOnlyService() *Service {
	return &Service{
		Paths:       s.Paths,
		Config:      s.CurrentConfig(),
		Prompter:    s.Prompter,
		Now:         s.Now,
		LogN:        s.LogN,
		Spawn:       s.Spawn,
		Waiting:     s.Waiting,
		UserHome:    s.UserHome,
		sessionOnly: true,
	}
}

func sshStatDirty(dir string, st sshdir.State) []string {
	var dirty []string
	for rel, fs := range st.Files {
		info, err := os.Lstat(filepath.Join(dir, filepath.FromSlash(rel)))
		if err != nil || !info.Mode().IsRegular() || info.Size() != fs.Size || !info.ModTime().Equal(fs.ModTime) {
			dirty = append(dirty, rel)
		}
	}
	return dirty
}

type sshAutoWords struct {
	done, failed, cmd string
}

var (
	sshPushWords = sshAutoWords{done: "pushed", failed: "could not be pushed", cmd: "status"}
	sshPullWords = sshAutoWords{done: "pulled", failed: "could not be written", cmd: "pull"}
)

func sshPushLeft(files []SSHFileResult, dirty []string) []SSHFileResult {
	var left []SSHFileResult
	for _, f := range files {
		if slices.Contains(dirty, f.Rel) && f.Action != SSHPushed && f.Action != SSHNoChange {
			left = append(left, f)
		}
	}
	return left
}

func sshAutoHint(left []SSHFileResult, w sshAutoWords) string {
	switch len(left) {
	case 0:
		return ""
	case 1:
		return fmt.Sprintf("ssh: %s %s, run creds ssh %s", left[0].Rel, sshAutoReason(left[0], w), w.cmd)
	}
	return fmt.Sprintf("ssh: %d files not %s, run creds ssh %s", len(left), w.done, w.cmd)
}

func sshAutoReason(f SSHFileResult, w sshAutoWords) string {
	switch {
	case f.Action == SSHFailed:
		return w.failed
	case f.Change == SSHConflict:
		return "changed here and in the vault"
	case f.Change == SSHLocalDeleted:
		return "deleted here"
	case f.Change == SSHRemoteDeleted:
		return "deleted in the vault"
	}
	return "changed in the vault"
}
