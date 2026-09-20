package tui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
)

type fieldList struct {
	fields   []field
	cursor   int
	revealed bool
}

func (l fieldList) move(delta int) fieldList {
	next := moveCursor(l.cursor, delta, len(l.fields))
	if next != l.cursor {
		l.cursor, l.revealed = next, false
	}
	return l
}

func (l fieldList) toggle(path string) (fieldList, tea.Cmd) {
	if l.revealed || len(l.fields) == 0 {
		l.revealed = false
		return l, nil
	}
	return l, send(revealMsg{path})
}

func (l fieldList) view(width, rows int) string {
	var b strings.Builder
	start, end := window(l.cursor, rows, len(l.fields))
	w := widestName(l.fields)
	for i := start; i < end; i++ {
		b.WriteString(pickRow(width, i == l.cursor, func(t theme) string {
			return fieldRow(t, l.fields[i], w, l.revealed && i == l.cursor)
		}))
	}
	return b.String()
}
