package tui

import (
	"fmt"
	"slices"

	tea "charm.land/bubbletea/v2"
	"github.com/elliot40404/creds/internal/app"
)

type (
	sshOpenMsg   struct{}
	sshLoadMsg   struct{}
	sshPushMsg   struct{ rel string }
	sshPullMsg   struct{ rel string }
	sshDiffMsg   struct{ file app.SSHFileStatus }
	sshStatusMsg struct {
		errResult
		status app.SSHStatus
		open   bool
	}
	sshDoneMsg struct {
		errResult
		pull   bool
		files  []app.SSHFileResult
		status app.SSHStatus
	}
)

func (sshOpenMsg) apply(m Model) (tea.Model, tea.Cmd) { return m.loadSSH(true) }
func (sshLoadMsg) apply(m Model) (tea.Model, tea.Cmd) { return m.loadSSH(false) }
func (msg sshDiffMsg) screen() Screen                 { return newSSHDiff(msg.file) }

func (msg sshPushMsg) apply(m Model) (tea.Model, tea.Cmd) {
	b, o := m.opts.Backend, app.SSHPushOptions{AskConflict: onlyRel(msg.rel)}
	return m.afterSync().start(opAction, func() tea.Msg {
		res, err := b.SSHPush(o)
		return sshRan(b, false, res.Files, err)
	})
}

func (msg sshPullMsg) apply(m Model) (tea.Model, tea.Cmd) {
	b, o := m.opts.Backend, app.SSHPullOptions{AskConflict: onlyFile(msg.rel), AskDelete: onlyFile(msg.rel)}
	return m.start(opAction, func() tea.Msg {
		res, err := b.SSHPull(o)
		return sshRan(b, true, res.Files, err)
	})
}

func onlyRel(rel string) func(string) (bool, error) {
	if rel == "" {
		return nil
	}
	return func(r string) (bool, error) { return r == rel, nil }
}

func onlyFile(rel string) func(app.SSHFileStatus) (bool, error) {
	if rel == "" {
		return nil
	}
	return func(f app.SSHFileStatus) (bool, error) { return f.Rel == rel, nil }
}

func sshRan(b Backend, pull bool, files []app.SSHFileResult, err error) tea.Msg {
	msg := sshDoneMsg{errResult: errResult{err}, pull: pull, files: files}
	if err == nil {
		msg.status, msg.err = b.SSHStatus()
	}
	return msg
}

func (m Model) loadSSH(open bool) (Model, tea.Cmd) {
	b := m.opts.Backend
	return m.start(opAction, func() tea.Msg {
		st, err := b.SSHStatus()
		return sshStatusMsg{errResult{err}, st, open}
	})
}

func (m Model) sshLoaded(msg sshStatusMsg) (tea.Model, tea.Cmd, bool) {
	if !msg.open {
		return m.done(msg.err, "press r to try again", m.toOwner(msg))
	}
	return m.done(msg.err, "run creds ssh status", func(m Model) (tea.Model, tea.Cmd) {
		return m.push(newSSH(msg.status)), nil
	})
}

func (m Model) sshDone(msg sshDoneMsg) (tea.Model, tea.Cmd, bool) {
	owner := m.op.owner
	m = m.finish()
	if msg.err != nil {
		return m.fail(firstLine(msg.err), "press r to refresh, or run creds ssh status"), nil, true
	}
	m, _ = m.sendTo(owner, msg)
	cmd := m.reload()
	if !msg.pull {
		cmd = tea.Batch(cmd, m.armSync())
	}
	if i := slices.IndexFunc(msg.files, func(f app.SSHFileResult) bool { return f.Err != nil }); i >= 0 {
		f := msg.files[i]
		return m.fail(fmt.Errorf("%s: %w", f.Rel, firstLine(f.Err)), "press r to refresh and try again"), cmd, true
	}
	return m.note("%s", sshSummary(msg.pull, msg.files)), cmd, true
}

func sshSummary(pull bool, files []app.SSHFileResult) string {
	verb, wrote := "push", []app.SSHAction{app.SSHPushed}
	if pull {
		verb, wrote = "pull", []app.SSHAction{app.SSHCreated, app.SSHUpdated, app.SSHDeleted}
	}
	n, kept := 0, 0
	for _, f := range files {
		switch {
		case slices.Contains(wrote, f.Action):
			n++
		case f.Action == app.SSHKeptConflict, f.Action == app.SSHNotAsked:
			kept++
		}
	}
	s := "nothing to " + verb
	if n > 0 {
		s = fmt.Sprintf("%sed %d files", verb, n)
	}
	if kept > 0 {
		s += fmt.Sprintf(", %d left for you to pick", kept)
	}
	return s
}
