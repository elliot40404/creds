package tui

import (
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/elliot40404/creds/internal/fail"
)

type sshFormKind int

const (
	sshFormHost sshFormKind = iota
	sshFormKey
	sshFormKeygen
)

type sshField struct {
	label  string
	hint   string
	input  textInput
	toggle bool
	on     bool
}

type sshForm struct {
	kind   sshFormKind
	title  string
	fields []sshField
	focus  int
	errMsg string
	fix    string
	saving bool
}

func (f sshForm) typing() bool {
	return true
}

func (f sshForm) keys() []keyHelp {
	return []keyHelp{
		{"tab down", "next field"},
		{"ctrl+s", "save"},
		{"esc", "cancel"},
		{"shift+tab up", "previous field"},
		{"enter", "next, save on last"},
		{"space", "switch on or off"},
		{"ctrl+u", "clear field"},
	}
}

func (f sshForm) Update(msg tea.Msg) (Screen, tea.Cmd) {
	switch msg := msg.(type) {
	case FormError:
		e := fail.Classify(msg.Err)
		f.saving, f.errMsg, f.fix = false, firstLine(msg.Err).Error(), e.Hint
	case tea.KeyPressMsg:
		if !f.saving {
			return f.key(msg)
		}
	case tea.PasteMsg:
		if !f.saving {
			f = f.edited(msg)
		}
	}
	return f, nil
}

func (f sshForm) key(key tea.KeyPressMsg) (Screen, tea.Cmd) {
	n := len(f.fields)
	switch key.String() {
	case "esc":
		f.wipe()
		return f, send(popMsg{})
	case "tab", "down":
		f.focus = (f.focus + 1) % n
	case "shift+tab", "up":
		f.focus = (f.focus + n - 1) % n
	case "enter":
		if f.focus < n-1 {
			f.focus++
			return f, nil
		}
		return f.submit()
	case "ctrl+s":
		return f.submit()
	case "space", "left", "right":
		if f.fields[f.focus].toggle {
			return f.flip(), nil
		}
		return f.edited(key), nil
	default:
		return f.edited(key), nil
	}
	return f, nil
}

func (f sshForm) edited(msg tea.Msg) sshForm {
	if f.fields[f.focus].toggle {
		return f
	}
	f.fields = slices.Clone(f.fields)
	if f.fields[f.focus].input.update(msg) {
		f.errMsg, f.fix = "", ""
	}
	return f
}

func (f sshForm) submit() (Screen, tea.Cmd) {
	msg, ok := f.request()
	if !ok {
		return f, nil
	}
	f.saving, f.errMsg, f.fix = true, "", ""
	return f, send(msg)
}

func (f sshForm) val(i int) string {
	return strings.TrimSpace(f.fields[i].input.String())
}

func (f *sshForm) need(i int, msg string) bool {
	if f.val(i) != "" {
		return true
	}
	f.focus, f.errMsg, f.fix = i, msg, ""
	return false
}

func (f *sshForm) wipe() {
	f.fields = slices.Clone(f.fields)
	for i := range f.fields {
		if f.fields[i].input.secret {
			clear(f.fields[i].input.value)
			f.fields[i].input = newInput("", true)
		}
	}
}

func (f sshForm) View(width, _ int) string {
	var b strings.Builder
	b.WriteString(line(th.title.Render(f.title)))
	b.WriteString(line(""))
	for i := range f.fields {
		b.WriteString(f.fieldView(width, i))
	}
	b.WriteString(line(""))
	switch {
	case f.errMsg != "":
		b.WriteString(line(fit(width, th.err.Render(f.errMsg))))
		if f.fix != "" {
			b.WriteString(line(fit(width, th.hint.Render(f.fix))))
		}
	case f.saving:
		b.WriteString(line(th.dim.Render("saving")))
	}
	return b.String()
}

func (f sshForm) fieldView(width, i int) string {
	fl, focused := f.fields[i], i == f.focus
	value := fl.input.view(focused && !f.saving)
	switch {
	case fl.toggle:
		value = th.key.Render("< ") + th.text.Render(onOff(fl.on)) + th.key.Render(" >")
	case fl.input.empty():
		value += th.dim.Render(fl.hint)
	}
	style, marker := th.label, markerOff
	if focused {
		style, marker = th.title, th.marker.Render(markerOn)
	}
	return line(fit(width, marker+style.Render(padRight(fl.label, labelWidth))+" "+value))
}

func onOff(on bool) string {
	if on {
		return "on"
	}
	return "off"
}
