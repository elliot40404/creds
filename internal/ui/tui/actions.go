package tui

import (
	"errors"

	tea "charm.land/bubbletea/v2"
	"github.com/elliot40404/creds/internal/render"
	"github.com/elliot40404/creds/internal/vault"
)

type (
	openMsg struct{ path string }
	popMsg  struct{}
	quitMsg struct{}
	copyMsg struct {
		path, field, as string
		shell           render.Shell
	}
	shellPickMsg struct{ path, as string }
	deleteMsg    struct{ path string }
	confirmMsg   struct{ path string }
	syncMsg      struct{}
	lockMsg      struct{}
	addMsg       struct{}
	editMsg      struct{ path string }
	formatsMsg   struct{ path string }
	revealMsg    struct{ path string }
	historyMsg   struct{ entry vault.Entry }
	revisionMsg  struct {
		entry vault.Entry
		index int
	}
)

func send(msg tea.Msg) tea.Cmd {
	return func() tea.Msg { return msg }
}

var errBusy = errors.New("still working on the last action")

func (m Model) handle(msg tea.Msg) (tea.Model, tea.Cmd, bool) {
	if next, cmd, ok := m.handleResult(msg); ok {
		return next, cmd, true
	}
	run := dispatch(msg)
	if run == nil {
		return m, nil, false
	}
	if next, blocked := m.gate(msg); blocked {
		return next, nil, true
	}
	next, cmd := run(m)
	return next, cmd, true
}

func (m Model) gate(msg tea.Msg) (Model, bool) {
	switch msg.(type) {
	case popMsg:
		return m, false
	case syncMsg:
		if m.op.syncing() {
			return m.note("sync already running"), true
		}
	}
	if m.op.kind != opNone {
		return m.fail(errBusy, "wait a moment and try again"), true
	}
	return m, false
}

func (m Model) get(path string, wrap func(vault.Entry, error) tea.Msg) (Model, tea.Cmd) {
	b := m.opts.Backend
	return m.start(opAction, func() tea.Msg { return wrap(b.Get(path)) })
}

type copiedMsg struct {
	errResult
	path, label string
}

func (m Model) copy(msg copyMsg) (tea.Model, tea.Cmd) {
	cp := m.opts.Copy
	if cp == nil {
		return m.fail(errors.New("copy is not available"), "use creds copy "+msg.path), nil
	}
	return m.start(opAction, func() tea.Msg {
		label, err := cp(msg.path, msg.field, msg.as, msg.shell)
		return copiedMsg{errResult{err}, msg.path, label}
	})
}

func (m Model) copied(msg copiedMsg) (tea.Model, tea.Cmd, bool) {
	m = m.finish()
	cf, failed := errors.AsType[*CopyFailed](msg.err)
	switch {
	case failed:
		return m.push(newAsk(askShow(cf))), nil, true
	case msg.err != nil:
		return m.fail(firstLine(msg.err), "use creds get "+msg.path+" --show instead"), nil, true
	}
	return m.note("Copied %s", msg.label), nil, true
}

func (m Model) lock() (tea.Model, tea.Cmd) {
	if err := m.opts.Backend.Lock(); err != nil {
		return m.fail(err, "run creds lock"), nil
	}
	return m.stop()
}

func (m Model) openForm(orig *vault.Entry) (tea.Model, tea.Cmd) {
	h := m.opts.Hooks
	switch {
	case orig == nil && h.Add != nil:
		m.editing = nil
		return m.push(h.Add(m.paths())), nil
	case orig != nil && h.Edit != nil:
		m.editing = orig
		return m.push(h.Edit(orig.Clone())), nil
	case orig == nil:
		return m.fail(errors.New("adding in the TUI is not ready yet"), "use creds add"), nil
	}
	return m.fail(errors.New("editing in the TUI is not ready yet"), "use creds edit "+orig.Path), nil
}

func (m Model) save(res FormResult) (tea.Model, tea.Cmd) {
	if res.Canceled {
		m.editing = nil
		return m.pop()
	}
	b, orig := m.opts.Backend, m.editing
	return m.start(opAction, func() tea.Msg {
		var e vault.Entry
		var err error
		if orig == nil {
			e, err = b.Add(res.Entry)
		} else {
			e, err = b.Update(orig.Path, res.Entry)
		}
		return savedMsg{errResult{err}, e}
	})
}

func (m Model) paths() []string {
	l, ok := m.stack[0].screen.(listScreen)
	if !ok {
		return nil
	}
	return l.index.Paths()
}
