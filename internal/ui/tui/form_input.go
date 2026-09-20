package tui

import (
	"slices"
	"strings"
	"unicode"

	tea "charm.land/bubbletea/v2"
)

type textInput struct {
	value  []rune
	secret bool
	reveal bool
}

func newInput(value string, secret bool) textInput {
	t := textInput{secret: secret}
	t.insert(value)
	return t
}

func (t *textInput) update(msg tea.Msg) bool {
	switch msg := msg.(type) {
	case tea.PasteMsg:
		t.insert(msg.Content)
		return true
	case tea.KeyPressMsg:
		switch msg.String() {
		case "backspace":
			if len(t.value) > 0 {
				t.value = t.value[:len(t.value)-1]
			}
			return true
		case "ctrl+u":
			t.value = nil
			return true
		}
		if msg.Text != "" && msg.Mod&(tea.ModCtrl|tea.ModAlt) == 0 {
			t.insert(msg.Text)
			return true
		}
	}
	return false
}

func (t *textInput) insert(s string) {
	t.value = slices.Clip(t.value)
	for _, r := range s {
		if !unicode.IsControl(r) {
			t.value = append(t.value, r)
		}
	}
}

func (t textInput) String() string {
	return string(t.value)
}

func (t textInput) empty() bool {
	return len(t.value) == 0
}

func (t textInput) view(focused bool) string {
	s := th.text.Render(string(t.value))
	if t.secret && !t.reveal {
		s = th.masked.Render(strings.Repeat("*", len(t.value)))
	}
	if focused {
		s += th.marker.Render("_")
	}
	return s
}
