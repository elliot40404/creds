package tui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/elliot40404/creds/internal/render"
)

type shellsScreen struct {
	path   string
	as     string
	shells []render.Shell
	cursor int
}

func newShells(path, as string, def render.Shell) shellsScreen {
	return shellsScreen{path: path, as: as, shells: []render.Shell{def, def.Other()}}
}

func (s shellsScreen) keys() []keyHelp {
	return []keyHelp{{"up/down j/k", "move"}, {"1-9", "copy for that shell"}, {"enter", "copy"}, {"esc", "back"}}
}

func (s shellsScreen) names() []string {
	out := make([]string, len(s.shells))
	for i, sh := range s.shells {
		out[i] = string(sh)
		if i == 0 {
			out[i] += "  your default"
		}
	}
	return out
}

func (s shellsScreen) copy(i int) tea.Cmd {
	return send(copyMsg{path: s.path, as: s.as, shell: s.shells[i]})
}

func (s shellsScreen) Update(msg tea.Msg) (Screen, tea.Cmd) {
	key, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return s, nil
	}
	if i, ok := pickNumber(key, len(s.shells)); ok {
		return s, s.copy(i)
	}
	if dy := navDelta(key); dy != 0 {
		s.cursor = moveCursor(s.cursor, dy, len(s.shells))
		return s, nil
	}
	switch key.String() {
	case "enter":
		return s, s.copy(s.cursor)
	case "esc", "q":
		return s, send(popMsg{})
	}
	return s, nil
}

func (s shellsScreen) View(width, height int) string {
	var b strings.Builder
	b.WriteString(line(th.title.Render(s.path+" as "+s.as) + "  " + th.dim.Render("pick a shell for this copy")))
	b.WriteString(line(""))
	b.WriteString(choices(width, height-2, s.names(), s.cursor))
	return b.String()
}
