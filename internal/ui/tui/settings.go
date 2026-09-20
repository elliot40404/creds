package tui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/elliot40404/creds/internal/app"
	"github.com/elliot40404/creds/internal/config"
)

type settingsScreen struct {
	paths    app.PathReport
	rows     []settingRow
	cursor   int
	editing  bool
	adding   bool
	input    textInput
	preview  func(key, value string) (string, error)
	shownKey string
	shown    string
	out      string
	outErr   error
}

func newSettings(c ConfigBackend) settingsScreen {
	s := settingsScreen{paths: c.PathReport(), rows: buildRows(c), preview: c.ConfigPreview}
	return s.render()
}

func (s settingsScreen) reload(c ConfigBackend) settingsScreen {
	key, value := s.current().key, s.current().value
	s.paths, s.rows, s.editing, s.adding = c.PathReport(), buildRows(c), false, false
	s.cursor = 0
	for i, r := range s.rows {
		if r.key == key && (r.kind != settingTrust || r.value == value) {
			s.cursor = i
		}
	}
	return s.render()
}

func (s settingsScreen) current() settingRow {
	if len(s.rows) == 0 {
		return settingRow{}
	}
	return s.rows[s.cursor]
}

func (s settingsScreen) typing() bool {
	return s.editing || s.adding
}

func (s settingsScreen) keys() []keyHelp {
	if s.adding {
		return []keyHelp{{"enter", "next"}, {"ctrl+u", "clear"}, {"esc", "cancel"}}
	}
	if s.editing {
		return []keyHelp{{"enter", "save"}, {"ctrl+u", "clear"}, {"esc", "cancel"}}
	}
	change := keyHelp{"enter", "change"}
	if s.current().cycles() {
		change = keyHelp{"left/right h/l", "change"}
	}
	return []keyHelp{
		change,
		{"r", "back to default"},
		{"a", "add format override"},
		{"x", "remove"},
		{"up/down j/k", "move"},
		{"esc q", "back"},
	}
}

func (s settingsScreen) Update(msg tea.Msg) (Screen, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.PasteMsg:
		if s.typing() {
			s.input.update(msg)
			return s.render(), nil
		}
	case tea.KeyPressMsg:
		next, cmd := s.keyPress(msg)
		if screen, ok := next.(settingsScreen); ok {
			return screen.render(), cmd
		}
		return next, cmd
	}
	return s, nil
}

func (s settingsScreen) keyPress(key tea.KeyPressMsg) (Screen, tea.Cmd) {
	if s.typing() {
		return s.typeKey(key)
	}
	if dy := navDelta(key); dy != 0 {
		s.cursor = moveCursor(s.cursor, dy, len(s.rows))
		return s, nil
	}
	switch key.String() {
	case "left", "h":
		return s.cycle(-1)
	case "right", "l":
		return s.cycle(1)
	case "enter":
		if s.current().editable() {
			s.editing, s.input = true, newInput(s.current().editValue(), s.current().kind == settingRemote)
		}
	case "a":
		s.adding, s.input = true, newInput(config.FormatPrefix, false)
	case "r":
		if r := s.current(); r.kind == settingConfig && r.value != r.def {
			return s, send(askMsg{
				title:  "Put " + r.key + " back to " + r.def + "?",
				detail: "the current value " + r.value + " is thrown away",
				then:   configSetMsg{key: r.key, value: r.def},
			})
		}
	case "x":
		return s.remove()
	case "esc", "q":
		return s, send(popMsg{})
	}
	return s, nil
}

func (s settingsScreen) cycle(delta int) (Screen, tea.Cmd) {
	r := s.current()
	if !r.cycles() {
		return s, nil
	}
	next, err := config.Next(r.key, r.value, delta)
	if err != nil {
		return s, nil
	}
	return s, send(configSetMsg{key: r.key, value: next, quiet: true})
}

func (s settingsScreen) remove() (Screen, tea.Cmd) {
	r := s.current()
	if !r.removable() {
		return s, nil
	}
	switch r.kind {
	case settingRemote:
		return s, send(askMsg{
			title:  "Remove the sync remote " + r.value + "?",
			detail: "sync stops until the url is typed again, and creds never shows the login part of it",
			then:   remoteRemoveMsg{},
		})
	case settingTrust:
		return s, send(askMsg{
			title:  "Forget " + r.value + "?",
			detail: "the next creds env or creds run in that project asks to trust the file again",
			then:   untrustMsg{file: r.value},
		})
	case settingConfig:
		return s, send(askMsg{
			title: "Remove " + r.key + "?",
			then:  configUnsetMsg{key: r.key},
		})
	}
	return s, nil
}

func (s settingsScreen) typeKey(key tea.KeyPressMsg) (Screen, tea.Cmd) {
	switch key.String() {
	case "esc":
		s.editing, s.adding, s.input = false, false, textInput{}
		return s, nil
	case "enter":
		return s.submit()
	}
	s.input.update(key)
	return s, nil
}

func (s settingsScreen) submit() (Screen, tea.Cmd) {
	value := s.input.String()
	if s.adding {
		s.adding, s.editing = false, true
		s.rows = append(s.rows, settingRow{key: value, doc: previewHint})
		s.cursor = len(s.rows) - 1
		s.input = newInput("", false)
		return s, nil
	}
	s.editing = false
	if r := s.current(); r.kind == settingRemote {
		if value == "" {
			return s, nil
		}
		return s, send(remoteSetMsg{url: r.submitValue(value)})
	}
	return s, send(configSetMsg{key: s.current().key, value: value})
}

func (s settingsScreen) editHint() string {
	r := s.current()
	if !s.editing || r.kind != settingRemote {
		return ""
	}
	if r.raw == "" {
		return remoteEditHint
	}
	return remoteEditHint + ", now " + r.value + " with any login hidden"
}

func (s settingsScreen) previewValue() string {
	if !strings.HasPrefix(s.current().key, config.FormatPrefix) || s.preview == nil {
		return ""
	}
	if s.editing {
		return s.input.String()
	}
	return s.current().value
}

func (s settingsScreen) render() settingsScreen {
	key, value := s.current().key, s.previewValue()
	if key == s.shownKey && value == s.shown {
		return s
	}
	s.shownKey, s.shown, s.out, s.outErr = key, value, "", nil
	if value != "" {
		s.out, s.outErr = s.preview(key, value)
	}
	return s
}
