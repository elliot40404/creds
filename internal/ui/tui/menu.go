package tui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/elliot40404/creds/internal/render"
)

type formatsScreen struct {
	path   string
	names  []string
	cursor int
	shell  render.Shell
}

func newFormats(path string, names []string, sh render.Shell) formatsScreen {
	return formatsScreen{path: path, names: names, shell: sh}
}

func (f formatsScreen) keys() []keyHelp {
	return []keyHelp{
		{"up/down j/k", "move"},
		{"1-9", "copy for " + string(f.shell)},
		{"shift+1-9", "copy for " + string(f.shell.Other())},
		{"enter", "copy, asking for a shell"},
		{"esc", "back"},
	}
}

func (f formatsScreen) Update(msg tea.Msg) (Screen, tea.Cmd) {
	key, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return f, nil
	}
	if i, ok := pickNumber(key, len(f.names)); ok {
		return f, send(copyMsg{path: f.path, as: f.names[i]})
	}
	if i, ok := pickShiftNumber(key, len(f.names)); ok {
		return f, send(copyMsg{path: f.path, as: f.names[i], shell: f.shell.Other()})
	}
	if dy := navDelta(key); dy != 0 {
		f.cursor = moveCursor(f.cursor, dy, len(f.names))
		return f, nil
	}
	switch key.String() {
	case "enter":
		if len(f.names) > 0 {
			return f, send(shellPickMsg{path: f.path, as: f.names[f.cursor]})
		}
	case "esc", "q":
		return f, send(popMsg{})
	}
	return f, nil
}

func (f formatsScreen) View(width, height int) string {
	var b strings.Builder
	b.WriteString(line(th.title.Render(f.path) + "  " + th.dim.Render("numbers copy for "+string(f.shell)+", shift for "+string(f.shell.Other()))))
	b.WriteString(line(""))
	if len(f.names) == 0 {
		b.WriteString(line(th.dim.Render("no formats for this engine")))
	}
	b.WriteString(choices(width, height-2, f.names, f.cursor))
	return b.String()
}
