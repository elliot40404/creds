package tui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/elliot40404/creds/internal/safetext"
)

type CopyFailed struct {
	Label string
	Value string
	Err   error
}

func (c *CopyFailed) Error() string { return c.Err.Error() }

func (c *CopyFailed) Unwrap() error { return c.Err }

type (
	showValueMsg struct{ label, value string }
	printMsg     struct{ value string }
)

func askShow(cf *CopyFailed) askMsg {
	return askMsg{
		title:  "clipboard failed: " + firstLine(cf.Err).Error(),
		detail: "show " + cf.Label + " here to copy by hand?",
		then:   showValueMsg{cf.Label, cf.Value},
	}
}

type valueScreen struct {
	label string
	value string
}

func newValue(msg showValueMsg) valueScreen {
	return valueScreen(msg)
}

func (v valueScreen) keys() []keyHelp {
	return []keyHelp{{"p", "print after quit"}, {"esc", "back"}}
}

func (v valueScreen) Update(msg tea.Msg) (Screen, tea.Cmd) {
	key, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return v, nil
	}
	switch key.String() {
	case "p":
		return v, send(printMsg{v.value})
	case "esc", "q":
		return v, send(popMsg{})
	}
	return v, nil
}

func (v valueScreen) View(width, _ int) string {
	var b strings.Builder
	b.WriteString(line(th.title.Render(v.label)))
	b.WriteString(line(""))
	b.WriteString(line(ansi.Hardwrap(safetext.Text(v.value), max(width, 1), true)))
	b.WriteString(line(""))
	b.WriteString(line(th.dim.Render("select to copy · p print after quit · esc back")))
	return b.String()
}
