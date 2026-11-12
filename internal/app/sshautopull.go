package app

import (
	"context"
	"errors"

	"github.com/elliot40404/creds/internal/gitsync"
	"github.com/elliot40404/creds/internal/session"
	"github.com/elliot40404/creds/internal/sshdir"
)

func (s *Service) SSHAuto() string {
	pulled := s.SSHAutoPull()
	pushed := s.SSHAutoPush()
	if pulled != "" {
		return pulled
	}
	return pushed
}

func (s *Service) SSHAutoPull() string {
	st, err := gitsync.LoadState(s.Paths.State())
	if err != nil || !st.SSHPull {
		return ""
	}
	hint, done := s.sshSafePull()
	if done {
		s.setSSHPull(false)
	}
	return hint
}

func (s *Service) sshSafePull() (string, bool) {
	st, err := sshdir.LoadState(s.Paths.SSHState())
	if err != nil || !s.sshAutoOn(st) {
		return "", err == nil
	}
	c := s.sessionOnlyService()
	res, err := c.SSH().Pull(SSHPullOptions{})
	for _, w := range c.takeWarnings() {
		s.warn(w)
	}
	switch {
	case errors.Is(err, session.ErrNoSession):
		return "", false
	case err != nil:
		return "ssh: auto pull failed, run creds ssh pull", true
	}
	return sshAutoHint(sshPullLeft(res.Files), sshPullWords), true
}

func sshPullLeft(files []SSHFileResult) []SSHFileResult {
	var left []SSHFileResult
	for _, f := range files {
		if f.Action == SSHKeptConflict || f.Action == SSHNotAsked || f.Action == SSHFailed {
			left = append(left, f)
		}
	}
	return left
}

func (s *Service) runSync(ctx context.Context, sy *gitsync.Syncer) (gitsync.SyncResult, error) {
	res, err := sy.Sync(ctx)
	if res == gitsync.FastForwarded || res == gitsync.Merged {
		s.markSSHPull()
	}
	return res, err
}

func (s *Service) markSSHPull() {
	st, err := sshdir.LoadState(s.Paths.SSHState())
	if err == nil && s.sshAutoOn(st) {
		s.setSSHPull(true)
	}
}

func (s *Service) setSSHPull(on bool) {
	_, _ = gitsync.UpdateState(s.Paths.State(), func(st *gitsync.State) (bool, error) {
		if st.SSHPull == on {
			return false, nil
		}
		st.SSHPull = on
		return true, nil
	})
}
