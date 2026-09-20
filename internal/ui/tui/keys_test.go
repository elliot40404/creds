package tui

import (
	tea "charm.land/bubbletea/v2"
)

var (
	enter    = tea.KeyPressMsg{Code: tea.KeyEnter}
	down     = tea.KeyPressMsg{Code: tea.KeyDown}
	up       = tea.KeyPressMsg{Code: tea.KeyUp}
	back     = tea.KeyPressMsg{Code: tea.KeyBackspace}
	esc      = tea.KeyPressMsg{Code: tea.KeyEscape}
	tabKey   = tea.KeyPressMsg{Code: tea.KeyTab}
	rightKey = tea.KeyPressMsg{Code: tea.KeyRight}
	leftKey  = tea.KeyPressMsg{Code: tea.KeyLeft}
)

func ch(r rune) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: r, Text: string(r)}
}

func ctrl(r rune) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: r, Mod: tea.ModCtrl}
}

func text(s string) []tea.KeyPressMsg {
	var out []tea.KeyPressMsg
	for _, r := range s {
		out = append(out, ch(r))
	}
	return out
}

func keys(s Screen, msgs ...tea.KeyPressMsg) (Screen, tea.Msg) {
	var last tea.Msg
	for _, k := range msgs {
		var cmd tea.Cmd
		s, cmd = s.Update(k)
		last = nil
		if cmd != nil {
			last = cmd()
		}
	}
	return s, last
}

func formSteps(s Screen, steps ...any) (Screen, tea.Msg) {
	var last tea.Msg
	for _, st := range steps {
		var ks []tea.KeyPressMsg
		switch v := st.(type) {
		case string:
			ks = text(v)
		case tea.KeyPressMsg:
			ks = []tea.KeyPressMsg{v}
		}
		s, last = keys(s, ks...)
	}
	return s, last
}

func repeat(k tea.KeyPressMsg, n int) []tea.KeyPressMsg {
	out := make([]tea.KeyPressMsg, n)
	for i := range out {
		out[i] = k
	}
	return out
}
