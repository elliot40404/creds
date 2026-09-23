package tui

import (
	"time"

	tea "charm.land/bubbletea/v2"
)

type (
	autoSyncMsg   struct{}
	autoSyncedMsg struct {
		errResult
		result string
	}
)

func (m Model) armSync() tea.Cmd {
	if m.opts.Config == nil {
		return nil
	}
	return tea.Tick(m.opts.Config.SyncAfter(), func(_ time.Time) tea.Msg { return autoSyncMsg{} })
}

func (m Model) syncIfDue() tea.Cmd {
	c := m.opts.Config
	if c == nil {
		return nil
	}
	return func() tea.Msg {
		if c.SyncDue() {
			return autoSyncMsg{}
		}
		return nil
	}
}

func (m Model) autoSync() (Model, tea.Cmd) {
	if !m.op.idle() {
		return m, m.armSync()
	}
	b := m.opts.Backend
	return m.start(opAutoSync, func() tea.Msg {
		res, err := b.Sync()
		return autoSyncedMsg{errResult{err}, res}
	})
}

func (m Model) autoSynced(msg autoSyncedMsg) (Model, tea.Cmd) {
	m = m.finish().syncResult(msg.result, msg.err, false)
	return m, m.reload()
}
