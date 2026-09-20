package tui

import (
	"time"

	tea "charm.land/bubbletea/v2"
)

const (
	idleFloor = time.Second
	idleCeil  = time.Minute
)

type sessionWatcher interface {
	SessionLeft() time.Duration
}

type sessionLeftMsg struct{ left time.Duration }

func (m Model) watch() (Model, tea.Cmd) {
	w, ok := m.opts.Backend.(sessionWatcher)
	if !ok || m.watching {
		return m, nil
	}
	m.watching = true
	return m, watchAfter(w, idleFloor)
}

func watchAfter(w sessionWatcher, d time.Duration) tea.Cmd {
	return tea.Tick(d, func(time.Time) tea.Msg { return sessionLeftMsg{w.SessionLeft()} })
}

func (m Model) sessionLeft(msg sessionLeftMsg) (Model, tea.Cmd) {
	if msg.left <= 0 {
		m = m.blank()
	}
	w, ok := m.opts.Backend.(sessionWatcher)
	if !ok || !m.showsSecrets() {
		m.watching = false
		return m, nil
	}
	return m, watchAfter(w, min(max(msg.left, idleFloor), idleCeil))
}

func (m Model) blank() Model {
	stack := make([]frame, 0, len(m.stack))
	for _, f := range m.stack {
		switch s := f.screen.(type) {
		case valueScreen:
			continue
		case detailScreen:
			s.list.revealed = false
			f.screen = s
		case revisionScreen:
			s.list.revealed = false
			f.screen = s
		}
		stack = append(stack, f)
	}
	m.stack = stack
	return m
}

func (m Model) showsSecrets() bool {
	for _, f := range m.stack {
		switch s := f.screen.(type) {
		case valueScreen:
			return true
		case detailScreen:
			if s.list.revealed {
				return true
			}
		case revisionScreen:
			if s.list.revealed {
				return true
			}
		}
	}
	return false
}
