package tui

import (
	"errors"
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/elliot40404/creds/internal/app"
	"github.com/elliot40404/creds/internal/fail"
	"github.com/elliot40404/creds/internal/search"
	"github.com/elliot40404/creds/internal/vault"
)

type errResult struct{ err error }

func (r errResult) failure() error { return r.err }

type (
	entryMsg struct {
		errResult
		entry vault.Entry
	}
	itemsMsg struct {
		errResult
		items    []search.Summary
		warnings []string
	}
	statusMsg struct {
		status app.SyncStatus
		err    error
	}
	deletedMsg struct {
		errResult
		path string
	}
	syncedMsg struct {
		errResult
		result string
	}
	savedMsg struct {
		errResult
		entry vault.Entry
	}
	revealedMsg struct {
		errResult
		entry vault.Entry
	}
	editEntryMsg struct {
		errResult
		entry vault.Entry
	}
	formatListMsg struct {
		errResult
		path  string
		names []string
	}
	shellVariesMsg struct {
		errResult
		path   string
		as     string
		varies bool
	}
)

func (m Model) handleResult(msg tea.Msg) (tea.Model, tea.Cmd, bool) {
	switch msg := msg.(type) {
	case itemsMsg:
		return m.setItems(msg), nil, true
	case statusMsg:
		m.status, m.stErr = msg.status, msg.err
		return m, nil, true
	case autoSyncMsg:
		next, cmd := m.autoSync()
		return next, cmd, true
	case autoSyncedMsg:
		next, cmd := m.autoSynced(msg)
		return next, cmd, true
	case lockedMsg:
		return m.locked(msg.retry), nil, true
	case PasswordResult:
		next, cmd := m.unlock(msg)
		return next, cmd, true
	case unlockedMsg:
		next, cmd := m.unlocked(msg)
		return next, cmd, true
	case entryMsg:
		if msg.err != nil {
			return m.finish().fail(msg.err, "press r in the list to reload"), m.loadItems(), true
		}
		return m.finish().push(newDetail(msg.entry)), nil, true
	case deletedMsg:
		return m.deleted(msg)
	case syncedMsg:
		return m.synced(msg)
	case savedMsg:
		return m.saved(msg)
	case editEntryMsg:
		return m.done(msg.err, "press esc and open the entry again", func(m Model) (tea.Model, tea.Cmd) {
			return m.openForm(&msg.entry)
		})
	case revealedMsg:
		next, cmd, ok := m.done(msg.err, "press esc and open the entry again", m.toOwner(msg))
		if msg.err != nil {
			return next, cmd, ok
		}
		watched, watch := next.(Model).watch()
		return watched, tea.Batch(cmd, watch), ok
	case sessionLeftMsg:
		next, cmd := m.sessionLeft(msg)
		return next, cmd, true
	case unmaskedMsg:
		return m.done(msg.err, "press ctrl+r to try again", m.toOwner(msg))
	case formatListMsg:
		return m.done(msg.err, "formats need a database entry with a host", func(m Model) (tea.Model, tea.Cmd) {
			return m.push(newFormats(msg.path, msg.names, m.opts.Backend.RenderShell())), nil
		})
	case remoteCheckedMsg:
		return m.remoteChecked(msg)
	case appliedMsg:
		return m.applied(msg)
	case copiedMsg:
		return m.copied(msg)
	case shellVariesMsg:
		return m.done(msg.err, "press esc and pick the format again", func(m Model) (tea.Model, tea.Cmd) {
			if !msg.varies {
				return m, send(copyMsg{path: msg.path, as: msg.as})
			}
			return m.push(newShells(msg.path, msg.as, m.opts.Backend.RenderShell())), nil
		})
	}
	return m, nil, false
}

func (m Model) done(err error, hint string, then func(Model) (tea.Model, tea.Cmd)) (tea.Model, tea.Cmd, bool) {
	m = m.finish()
	if err != nil {
		return m.fail(err, hint), nil, true
	}
	next, cmd := then(m)
	return next, cmd, true
}

func (m Model) toOwner(msg tea.Msg) func(Model) (tea.Model, tea.Cmd) {
	owner := m.op.owner
	return func(m Model) (tea.Model, tea.Cmd) { return m.sendTo(owner, msg) }
}

func (m Model) setItems(msg itemsMsg) Model {
	if msg.err != nil {
		return m.fail(msg.err, "press r to retry")
	}
	if l, ok := m.stack[0].screen.(listScreen); ok {
		m.stack[0].screen = l.setItems(msg.items)
	}
	if len(msg.warnings) > 0 {
		m = m.fail(errors.New("warning: "+strings.Join(msg.warnings, "; ")), "")
	}
	return m
}

func (m Model) deleted(msg deletedMsg) (tea.Model, tea.Cmd, bool) {
	m = m.finish()
	if msg.err != nil {
		return m.fail(msg.err, "run creds rm "+msg.path), nil, true
	}
	m.stack = m.stack[:1]
	return m.note("Deleted %s", msg.path), tea.Batch(m.reload(), m.armSync()), true
}

func (m Model) synced(msg syncedMsg) (tea.Model, tea.Cmd, bool) {
	m = m.finish()
	return m.syncResult(msg.result, msg.err, true), m.reload(), true
}

func (m Model) syncResult(result string, err error, loud bool) Model {
	var conflict *app.ConflictError
	switch {
	case errors.As(err, &conflict):
		return m.fail(fmt.Errorf("sync conflicts in %s", strings.Join(conflict.Paths, ", ")), "fix with creds resolve <path> --mine or --theirs")
	case err != nil:
		return m.syncFail(err)
	case loud:
		return m.note("Sync: %s", result)
	}
	return m
}

func (m Model) syncFail(err error) Model {
	e := fail.Classify(err)
	fix := e.Hint
	if fix == "" {
		fix = "check the remote with creds status, then press s"
	}
	return m.fail(fmt.Errorf("sync failed: %w", firstLine(e)), fix)
}

func (m Model) saved(msg savedMsg) (tea.Model, tea.Cmd, bool) {
	owner := m.op.owner
	m = m.finish()
	if msg.err != nil {
		next, cmd := m.sendTo(owner, FormError{Err: msg.err})
		return next, cmd, true
	}
	m = m.drop(owner)
	if m.editing != nil {
		m.stack[len(m.stack)-1].screen = newDetail(msg.entry)
	}
	m.editing = nil
	return m.note("Saved %s", msg.entry.Path), tea.Batch(m.reload(), m.armSync()), true
}

func (m Model) reload() tea.Cmd {
	return tea.Batch(m.loadItems(), m.loadStatus())
}

func (m Model) loadItems() tea.Cmd {
	b := m.opts.Backend
	return guard(func() tea.Msg {
		items, err := b.List()
		return itemsMsg{errResult{err}, items, b.Warnings()}
	})
}

func (m Model) loadStatus() tea.Cmd {
	b := m.opts.Backend
	return func() tea.Msg {
		st, err := b.SyncStatus()
		return statusMsg{st, err}
	}
}

func firstLine(err error) error {
	msg, _, _ := strings.Cut(err.Error(), "\n")
	return errors.New(msg)
}
