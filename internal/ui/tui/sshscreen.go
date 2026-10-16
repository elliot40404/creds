package tui

import (
	"path/filepath"

	tea "charm.land/bubbletea/v2"
	"github.com/elliot40404/creds/internal/app"
)

type sshScreen struct {
	status app.SSHStatus
	cursor int
}

func newSSH(st app.SSHStatus) sshScreen {
	return sshScreen{status: st}
}

func (s sshScreen) keys() []keyHelp {
	return []keyHelp{
		{"p", "push changes here to the vault"},
		{"l", "pull vault changes here"},
		{"enter d", "diff"},
		{"a", "add a host"},
		{"i", "add a key file"},
		{"g", "make a new key"},
		{"P", "conflict: keep this file, replace the vault copy"},
		{"L", "conflict or delete: take the vault side here"},
		{"r", "refresh"},
		{"up/down j/k", "move"},
		{"esc q", "back"},
	}
}

func (s sshScreen) Update(msg tea.Msg) (Screen, tea.Cmd) {
	switch msg := msg.(type) {
	case sshStatusMsg:
		return s.reload(msg.status), nil
	case sshDoneMsg:
		return s.reload(msg.status), nil
	case tea.KeyPressMsg:
		return s.key(msg)
	}
	return s, nil
}

func (s sshScreen) reload(st app.SSHStatus) sshScreen {
	s.status = st
	s.cursor = moveCursor(s.cursor, 0, len(st.Files))
	return s
}

func (s sshScreen) key(key tea.KeyPressMsg) (Screen, tea.Cmd) {
	if dy := navDelta(key); dy != 0 {
		s.cursor = moveCursor(s.cursor, dy, len(s.status.Files))
		return s, nil
	}
	switch key.String() {
	case "esc", "q":
		return s, send(popMsg{})
	case "p":
		return s, send(sshPushMsg{})
	case "l":
		return s, send(sshPullMsg{})
	case "r":
		return s, send(sshLoadMsg{})
	case "a":
		return s, send(sshHostFormMsg{keys: sshKeyNames(s.status)})
	case "i":
		return s, send(sshKeyFormMsg{})
	case "g":
		return s, send(sshKeygenFormMsg{})
	}
	return s, s.fileKey(key.String())
}

func (s sshScreen) fileKey(key string) tea.Cmd {
	f, ok := s.current()
	if !ok {
		return nil
	}
	switch key {
	case "enter", "d":
		return send(sshDiffMsg{f})
	case "P":
		return keepHere(f)
	case "L":
		return takeVault(f, s.status.Dir)
	}
	return nil
}

func (s sshScreen) current() (app.SSHFileStatus, bool) {
	if s.cursor >= len(s.status.Files) {
		return app.SSHFileStatus{}, false
	}
	return s.status.Files[s.cursor], true
}

func keepHere(f app.SSHFileStatus) tea.Cmd {
	if f.Change != app.SSHConflict || f.Disk == nil {
		return nil
	}
	return send(askMsg{
		title:  "Replace the vault copy of " + f.Rel + " with the file here?",
		detail: "it changed here and in the vault, the vault copy is overwritten",
		then:   sshPushMsg{rel: f.Rel},
	})
}

func takeVault(f app.SSHFileStatus, dir string) tea.Cmd {
	path := filepath.Join(dir, filepath.FromSlash(f.Rel))
	switch {
	case f.Vault == nil && (f.Change == app.SSHRemoteDeleted || f.Change == app.SSHConflict):
		return send(askMsg{title: "Delete " + f.Rel + " here?", detail: "it was deleted in the vault, " + path + " is removed", then: sshPullMsg{rel: f.Rel}})
	case f.Change == app.SSHConflict:
		return send(askMsg{title: "Replace " + f.Rel + " here with the vault copy?", detail: "it changed here and in the vault, " + path + " is overwritten", then: sshPullMsg{rel: f.Rel}})
	}
	return nil
}
