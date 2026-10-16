package tui

import (
	"slices"

	tea "charm.land/bubbletea/v2"
	"github.com/elliot40404/creds/internal/app"
	"github.com/elliot40404/creds/internal/sshdir"
)

type (
	sshHostFormMsg   struct{ keys []string }
	sshKeyFormMsg    struct{}
	sshKeygenFormMsg struct{}
	sshAddHostMsg    struct{ host app.SSHHost }
	sshAddKeyMsg     struct{ file, name string }
	sshKeygenMsg     struct{ gen app.SSHKeygen }
	sshAddedMsg      struct {
		errResult
		note      string
		status    app.SSHStatus
		statusErr error
	}
)

func (msg sshHostFormMsg) screen() Screen { return newHostForm(msg.keys) }
func (sshKeyFormMsg) screen() Screen      { return newKeyForm() }
func (sshKeygenFormMsg) screen() Screen   { return newKeygenForm() }

func (msg sshAddHostMsg) apply(m Model) (tea.Model, tea.Cmd) {
	b := m.opts.Backend
	return m.sshAdd(func() (string, error) {
		res, err := b.SSHAddHost(msg.host)
		return hostNote(res), err
	})
}

func (msg sshAddKeyMsg) apply(m Model) (tea.Model, tea.Cmd) {
	b := m.opts.Backend
	return m.sshAdd(func() (string, error) {
		res, err := b.SSHAddKey(msg.file, msg.name)
		return keyNote(res), err
	})
}

func (msg sshKeygenMsg) apply(m Model) (tea.Model, tea.Cmd) {
	b := m.opts.Backend
	return m.sshAdd(func() (string, error) {
		res, err := b.SSHKeygen(msg.gen)
		return "created " + res.Rel + " and " + res.Rel + ".pub, press a to add a host with it", err
	})
}

func (m Model) sshAdd(run func() (string, error)) (tea.Model, tea.Cmd) {
	b := m.opts.Backend
	return m.afterSync().start(opAction, func() tea.Msg {
		note, err := run()
		msg := sshAddedMsg{errResult: errResult{err}, note: note}
		if err == nil {
			msg.status, msg.statusErr = b.SSHStatus()
		}
		return msg
	})
}

func (m Model) sshAdded(msg sshAddedMsg) (tea.Model, tea.Cmd, bool) {
	owner := m.op.owner
	m = m.finish()
	if msg.err != nil {
		next, cmd := m.sendTo(owner, FormError{Err: msg.err})
		return next, cmd, true
	}
	m = m.drop(owner)
	cmd := tea.Batch(m.reload(), m.armSync())
	if msg.statusErr != nil {
		return m.fail(firstLine(msg.statusErr), "press r to refresh"), cmd, true
	}
	m, _ = m.sendTo(m.topID(), sshStatusMsg{status: msg.status})
	return m.note("%s", msg.note), cmd, true
}

func hostNote(res app.SSHAddHostResult) string {
	if res.Include != "" {
		return "created " + res.Rel + ", add to ~/.ssh/config: " + res.Include
	}
	return "created " + res.Rel + ", run ssh " + res.Alias + " to connect"
}

func keyNote(res app.SSHAddKeyResult) string {
	switch {
	case !slices.ContainsFunc(res.Files, func(f app.SSHFileResult) bool { return f.Action != app.SSHNoChange }):
		return res.Rel + " is already in ~/.ssh and the vault"
	case res.Unused:
		return "added " + res.Rel + ", no host uses it yet, press a to add one"
	}
	return "added " + res.Rel
}

func sshKeyNames(st app.SSHStatus) []string {
	var out []string
	for _, f := range st.Files {
		if f.Kind == sshdir.KindPrivateKey {
			out = append(out, f.Rel)
		}
	}
	return out
}
