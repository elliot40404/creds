package tui

import (
	"slices"

	tea "charm.land/bubbletea/v2"
)

type (
	unmaskMsg struct {
		row    int
		key    string
		toggle bool
	}
	unmaskedMsg struct {
		unmaskMsg
		errResult
	}
)

func toggles(r formRow) bool {
	return r.custom && r.kind == kindField
}

func (f formScreen) revealKey() (formScreen, tea.Cmd) {
	r := f.rows[f.focus]
	if r.input.secret && !r.input.reveal {
		return f, send(unmaskMsg{row: f.focus, key: r.key})
	}
	f.rows = slices.Clone(f.rows)
	f.rows[f.focus].input.reveal = false
	return f, nil
}

func (f formScreen) toggleKey() (formScreen, tea.Cmd) {
	r := f.rows[f.focus]
	if !toggles(r) {
		return f, nil
	}
	if r.input.secret {
		return f, send(unmaskMsg{row: f.focus, key: r.key, toggle: true})
	}
	f.rows = slices.Clone(f.rows)
	f.rows[f.focus].input.secret, f.rows[f.focus].input.reveal = true, false
	return f, nil
}

func (f formScreen) unmask(msg unmaskedMsg) formScreen {
	if msg.err != nil || msg.row >= len(f.rows) || f.rows[msg.row].key != msg.key {
		return f
	}
	f.rows = slices.Clone(f.rows)
	in := &f.rows[msg.row].input
	switch {
	case msg.toggle && toggles(f.rows[msg.row]):
		in.secret, in.reveal = false, false
	case !msg.toggle && in.secret:
		in.reveal = true
	}
	return f
}
