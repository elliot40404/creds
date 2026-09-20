package tui

import (
	"slices"

	tea "charm.land/bubbletea/v2"
)

type opKind int

const (
	opNone opKind = iota
	opAction
	opSync
	opAutoSync
)

type op struct {
	kind   opKind
	owner  int
	retry  tea.Cmd
	prompt int
}

func (o op) syncing() bool {
	return o.kind == opSync || o.kind == opAutoSync
}

func (o op) idle() bool {
	return o.kind == opNone && o.prompt == 0
}

type frame struct {
	id     int
	screen Screen
}

func (m Model) start(kind opKind, fn tea.Cmd) (Model, tea.Cmd) {
	m.op = op{kind: kind, owner: m.topID()}
	return m, guard(fn)
}

func (m Model) finish() Model {
	m.op = op{}
	return m
}

func (m Model) topID() int {
	return m.stack[len(m.stack)-1].id
}

func (m Model) index(id int) int {
	return slices.IndexFunc(m.stack, func(f frame) bool { return f.id == id })
}

func (m Model) drop(id int) Model {
	if i := m.index(id); i > 0 {
		m.stack = slices.Delete(slices.Clone(m.stack), i, i+1)
	}
	return m
}

func (m Model) sendTo(id int, msg tea.Msg) (Model, tea.Cmd) {
	i := m.index(id)
	if i < 0 {
		return m, nil
	}
	next, cmd := m.stack[i].screen.Update(msg)
	m.stack[i].screen = next
	return m, cmd
}
