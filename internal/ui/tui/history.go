package tui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/elliot40404/creds/internal/vault"
)

type historyScreen struct {
	entry  vault.Entry
	cursor int
}

func newHistory(e vault.Entry) historyScreen {
	return historyScreen{entry: e}
}

func (h historyScreen) keys() []keyHelp {
	return []keyHelp{
		{"enter", "open this version"},
		{"up/down j/k", "move"},
		{"esc", "back"},
	}
}

func (h historyScreen) Update(msg tea.Msg) (Screen, tea.Cmd) {
	key, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return h, nil
	}
	if i, ok := pickNumber(key, len(h.entry.History)); ok {
		return h, send(revisionMsg{h.entry, i})
	}
	if dy := navDelta(key); dy != 0 {
		h.cursor = moveCursor(h.cursor, dy, len(h.entry.History))
		return h, nil
	}
	switch key.String() {
	case "enter":
		if len(h.entry.History) > 0 {
			return h, send(revisionMsg{h.entry, h.cursor})
		}
	case "esc", "q":
		return h, send(popMsg{})
	}
	return h, nil
}

func (h historyScreen) View(width, height int) string {
	var b strings.Builder
	b.WriteString(line(th.title.Render(h.entry.Path) + "  " + th.dim.Render("previous versions, newest first")))
	b.WriteString(line(""))
	names := make([]string, len(h.entry.History))
	for i, r := range h.entry.History {
		names[i] = stamp(r.At) + "  " + machineOf(r)
	}
	if len(names) == 0 {
		b.WriteString(line(th.dim.Render("no previous versions kept, see vault.history")))
		return b.String()
	}
	b.WriteString(choices(width, height-2, names, h.cursor))
	return b.String()
}

func machineOf(r vault.Revision) string {
	if r.Machine == "" {
		return "unknown machine"
	}
	return r.Machine
}

type revisionScreen struct {
	path string
	rev  vault.Revision
	list fieldList
}

func newRevision(e vault.Entry, i int) revisionScreen {
	r := e.History[i]
	return revisionScreen{path: e.Path, rev: r, list: fieldList{fields: fieldsOf(e.WithRevision(r))}}
}

func (v revisionScreen) keys() []keyHelp {
	return []keyHelp{
		{"r", "reveal field"},
		{"up/down j/k", "move"},
		{"esc", "back"},
	}
}

func (v revisionScreen) Update(msg tea.Msg) (Screen, tea.Cmd) {
	if r, ok := msg.(revealedMsg); ok {
		v.list.revealed = r.entry.Path == v.path && len(v.list.fields) > 0
		return v, nil
	}
	key, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return v, nil
	}
	if dy := navDelta(key); dy != 0 {
		v.list = v.list.move(dy)
		return v, nil
	}
	switch key.String() {
	case "r":
		var cmd tea.Cmd
		v.list, cmd = v.list.toggle(v.path)
		return v, cmd
	case "esc", "q":
		return v, send(popMsg{})
	}
	return v, nil
}

func (v revisionScreen) View(width, height int) string {
	var b strings.Builder
	b.WriteString(line(th.title.Render(v.path) + "  " + th.badge.Render("old version")))
	b.WriteString(line(th.dim.Render(stamp(v.rev.At) + "  on " + machineOf(v.rev))))
	b.WriteString(line(""))
	if len(v.list.fields) == 0 {
		b.WriteString(line(th.dim.Render("this version had no values")))
		return b.String()
	}
	b.WriteString(v.list.view(width, height-3))
	return b.String()
}
