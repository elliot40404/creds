package tui

import (
	"errors"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/elliot40404/creds/internal/crypto"
)

type promptScreen struct {
	message string
	input   textInput
	errMsg  string
	waiting bool
}

func NewPrompt(message string) Screen {
	return promptScreen{message: message, input: newInput("", true)}
}

func (p promptScreen) typing() bool {
	return true
}

func (p promptScreen) keys() []keyHelp {
	return []keyHelp{{"enter", "unlock"}, {"ctrl+u", "clear"}, {"esc", "cancel"}}
}

func (p promptScreen) Update(msg tea.Msg) (Screen, tea.Cmd) {
	switch msg := msg.(type) {
	case PasswordError:
		p.waiting = false
		p.errMsg = passwordHint(msg.Err)
	case tea.KeyPressMsg:
		return p.key(msg)
	case tea.PasteMsg:
		if !p.waiting {
			p.input.update(msg)
		}
	}
	return p, nil
}

func (p promptScreen) key(key tea.KeyPressMsg) (Screen, tea.Cmd) {
	switch key.String() {
	case "esc":
		p.input = newInput("", true)
		return p, send(PasswordResult{Canceled: true})
	case "enter":
		if p.waiting {
			return p, nil
		}
		if p.input.empty() {
			p.errMsg = "password is empty. type your master password and press enter"
			return p, nil
		}
		pw := p.input.String()
		p.input = newInput("", true)
		p.waiting, p.errMsg = true, ""
		return p, send(PasswordResult{Password: pw})
	}
	if !p.waiting && p.input.update(key) {
		p.errMsg = ""
	}
	return p, nil
}

func passwordHint(err error) string {
	if errors.Is(err, crypto.ErrWrongSecret) {
		return "wrong password. try again, or press esc to quit"
	}
	return "unlock failed: " + firstLine(err).Error() + ". try again, or press esc and run creds unlock"
}

func (p promptScreen) View(int, int) string {
	var b strings.Builder
	b.WriteString(line(th.title.Render(p.message)))
	b.WriteString(line(""))
	b.WriteString(line(th.label.Render("password  ") + p.input.view(!p.waiting)))
	b.WriteString(line(""))
	switch {
	case p.errMsg != "":
		b.WriteString(line(th.err.Render(p.errMsg)))
	case p.waiting:
		b.WriteString(line(th.dim.Render("checking")))
	}
	return b.String()
}
