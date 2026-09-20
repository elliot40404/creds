package tui

import (
	tea "charm.land/bubbletea/v2"
)

type (
	askMsg struct {
		title  string
		detail string
		then   tea.Msg
	}
	confirmedMsg struct{ then tea.Msg }
)

type askScreen struct {
	title  string
	detail string
	then   tea.Msg
}

func newAsk(msg askMsg) askScreen {
	return askScreen(msg)
}

func (a askScreen) keys() []keyHelp {
	return []keyHelp{{"y", "yes"}, {"any other key", "keep as is"}}
}

func (a askScreen) Update(msg tea.Msg) (Screen, tea.Cmd) {
	key, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return a, nil
	}
	if key.String() == "y" {
		return a, send(confirmedMsg{then: a.then})
	}
	return a, send(popMsg{})
}

func (a askScreen) View(width, _ int) string {
	out := line(fit(width, th.danger.Render(a.title)))
	if a.detail != "" {
		out += line("") + line(fit(width, th.text.Render(a.detail)))
	}
	return out + line("") +
		line(th.key.Render("y")+th.label.Render(" yes   ")+th.key.Render("any other key")+th.label.Render(" keep as is"))
}
