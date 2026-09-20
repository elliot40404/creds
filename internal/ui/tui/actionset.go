package tui

import (
	tea "charm.land/bubbletea/v2"
	"github.com/elliot40404/creds/internal/vault"
)

type action interface {
	apply(m Model) (tea.Model, tea.Cmd)
}

type opener interface {
	screen() Screen
}

func dispatch(msg tea.Msg) func(Model) (tea.Model, tea.Cmd) {
	switch a := msg.(type) {
	case opener:
		return func(m Model) (tea.Model, tea.Cmd) { return m.push(a.screen()), nil }
	case action:
		return a.apply
	}
	return nil
}

func (popMsg) apply(m Model) (tea.Model, tea.Cmd)         { return m.pop() }
func (quitMsg) apply(m Model) (tea.Model, tea.Cmd)        { return m.stop() }
func (msg copyMsg) apply(m Model) (tea.Model, tea.Cmd)    { return m.copy(msg) }
func (lockMsg) apply(m Model) (tea.Model, tea.Cmd)        { return m.lock() }
func (addMsg) apply(m Model) (tea.Model, tea.Cmd)         { return m.openForm(nil) }
func (msg FormResult) apply(m Model) (tea.Model, tea.Cmd) { return m.save(msg) }

func (msg deleteMsg) screen() Screen {
	return newAsk(askMsg{title: "Delete this entry? This cannot be undone.", detail: "entry " + msg.path, then: confirmMsg(msg)})
}

func (msg historyMsg) screen() Screen  { return newHistory(msg.entry) }
func (msg revisionMsg) screen() Screen { return newRevision(msg.entry, msg.index) }
func (msg askMsg) screen() Screen      { return newAsk(msg) }

func (msg openMsg) apply(m Model) (tea.Model, tea.Cmd) {
	return m.get(msg.path, func(e vault.Entry, err error) tea.Msg { return entryMsg{errResult{err}, e} })
}

func (msg editMsg) apply(m Model) (tea.Model, tea.Cmd) {
	return m.get(msg.path, func(e vault.Entry, err error) tea.Msg { return editEntryMsg{errResult{err}, e} })
}

func (msg revealMsg) apply(m Model) (tea.Model, tea.Cmd) {
	return m.get(msg.path, func(e vault.Entry, err error) tea.Msg { return revealedMsg{errResult{err}, e} })
}

func (msg confirmMsg) apply(m Model) (tea.Model, tea.Cmd) {
	b := m.opts.Backend
	return m.start(opAction, func() tea.Msg {
		return deletedMsg{errResult{b.Delete(msg.path)}, msg.path}
	})
}

func (syncMsg) apply(m Model) (tea.Model, tea.Cmd) {
	b := m.opts.Backend
	return m.note("syncing").start(opSync, func() tea.Msg {
		res, err := b.Sync()
		return syncedMsg{errResult{err}, res}
	})
}

func (msg unmaskMsg) apply(m Model) (tea.Model, tea.Cmd) {
	b := m.opts.Backend
	return m.start(opAction, func() tea.Msg {
		_, err := b.List()
		return unmaskedMsg{msg, errResult{err}}
	})
}

func (msg formatsMsg) apply(m Model) (tea.Model, tea.Cmd) {
	b := m.opts.Backend
	return m.start(opAction, func() tea.Msg {
		names, err := b.Formats(msg.path)
		return formatListMsg{errResult{err}, msg.path, names}
	})
}

func (msg shellPickMsg) apply(m Model) (tea.Model, tea.Cmd) {
	b := m.opts.Backend
	return m.start(opAction, func() tea.Msg {
		varies, err := b.ShellVaries(msg.path, msg.as)
		return shellVariesMsg{errResult{err}, msg.path, msg.as, varies}
	})
}

func (settingsMsg) apply(m Model) (tea.Model, tea.Cmd)        { return m.openSettings() }
func (msg configSetMsg) apply(m Model) (tea.Model, tea.Cmd)   { return m.setConfig(msg) }
func (msg configUnsetMsg) apply(m Model) (tea.Model, tea.Cmd) { return m.unsetConfig(msg) }
func (msg remoteSetMsg) apply(m Model) (tea.Model, tea.Cmd)   { return m.checkRemote(msg) }
func (msg remoteSaveMsg) apply(m Model) (tea.Model, tea.Cmd)  { return m.setRemote(msg) }
func (remoteRemoveMsg) apply(m Model) (tea.Model, tea.Cmd)    { return m.removeRemote() }
func (msg untrustMsg) apply(m Model) (tea.Model, tea.Cmd)     { return m.untrust(msg) }
func (msg confirmedMsg) apply(m Model) (tea.Model, tea.Cmd)   { return m.confirmed(msg) }

func (msg showValueMsg) apply(m Model) (tea.Model, tea.Cmd) {
	return m.push(newValue(msg)).watch()
}

func (msg printMsg) apply(m Model) (tea.Model, tea.Cmd) {
	m.printOnQuit = msg.value
	return m.stop()
}
