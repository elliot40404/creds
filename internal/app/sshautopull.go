package app

import (
	"errors"

	"github.com/elliot40404/creds/internal/gitsync"
	"github.com/elliot40404/creds/internal/session"
	"github.com/elliot40404/creds/internal/sshdir"
)

func (x SSH) Auto() string {
	pulled := x.AutoPull()
	pushed := x.AutoPush()
	if pulled != "" {
		return pulled
	}
	return pushed
}

func (x SSH) AutoPull() string {
	st, err := gitsync.LoadState(x.s.Paths.State())
	if err != nil || !st.SSHPull {
		return ""
	}
	hint, done := x.safePull()
	if done {
		x.setPull(false)
	}
	return hint
}

func (x SSH) safePull() (string, bool) {
	st, err := sshdir.LoadState(x.s.Paths.SSHState())
	if err != nil || !x.autoOn(st) {
		return "", err == nil
	}
	c := x.sessionService()
	res, err := c.SSH().Pull(SSHPullOptions{})
	for _, w := range c.takeWarnings() {
		x.s.warn(w)
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

func (x SSH) markPull() {
	st, err := sshdir.LoadState(x.s.Paths.SSHState())
	if err == nil && x.autoOn(st) {
		x.setPull(true)
	}
}

func (x SSH) setPull(on bool) {
	_, _ = gitsync.UpdateState(x.s.Paths.State(), func(st *gitsync.State) (bool, error) {
		if st.SSHPull == on {
			return false, nil
		}
		st.SSHPull = on
		return true, nil
	})
}
