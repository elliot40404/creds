package tui

import (
	"strings"

	"github.com/elliot40404/creds/internal/app"
)

const labelWidth = 20

func (f formScreen) keys() []keyHelp {
	if f.picking {
		return []keyHelp{{"up/down j/k", "move"}, {"1-9", "pick that one"}, {"enter", "pick type"}, {"esc", "cancel"}}
	}
	return []keyHelp{
		{"tab", "complete path or next field"},
		{"down", "next field"},
		{"shift+tab/up", "previous field"},
		{"enter", "next, save on last"},
		{"ctrl+s", "save"},
		{"ctrl+n", "add custom field"},
		{"ctrl+t", "toggle secret on custom field"},
		{"ctrl+d", "remove custom field or param"},
		{"ctrl+r", "reveal focused secret"},
		{"left/right", "change engine"},
		{"ctrl+u", "clear field"},
		{"esc", "cancel"},
	}
}

func (f formScreen) View(width, height int) string {
	if f.picking {
		return f.pickView(width, height)
	}
	var b strings.Builder
	title := "Add " + string(f.typ)
	if f.edit {
		title = "Edit " + f.orig.Path
	}
	b.WriteString(line(th.title.Render(title)))
	b.WriteString(line(""))
	start, end := window(f.focus, height-5, len(f.rows))
	for i := start; i < end; i++ {
		b.WriteString(f.rowView(i))
	}
	b.WriteString(line(""))
	switch {
	case f.naming != nil:
		b.WriteString(line(th.label.Render("new field name  ") + f.naming.view(true)))
		if f.nameErr != "" {
			b.WriteString(line(th.err.Render(f.nameErr)))
		}
	case f.errMsg != "":
		b.WriteString(line(th.err.Render(f.errMsg)))
	case f.saving:
		b.WriteString(line(th.dim.Render("saving")))
	}
	return b.String()
}

func (f formScreen) pickView(width, height int) string {
	return line(th.title.Render("New entry")+"  "+th.dim.Render("pick a type, enter to continue")) +
		line("") + choices(width, height-2, app.TypeNames(), f.cursor)
}

func (f formScreen) rowView(i int) string {
	r := f.rows[i]
	focused := i == f.focus && f.naming == nil
	value := r.input.view(focused)
	if r.ghost != "" {
		value += th.dim.Render(r.ghost)
	}
	switch {
	case r.kind == kindEngine:
		value = th.key.Render("< ") + th.text.Render(f.engines[f.engine]) + th.key.Render(" >")
		if f.engineNote != "" {
			value += "  " + th.dim.Render(f.engineNote)
		}
	case r.keep && r.input.empty():
		value += th.dim.Render("(blank keeps current)")
	case r.kind == kindParam && r.input.empty():
		value += th.dim.Render("(blank removes it)")
	}
	label := r.key
	switch {
	case r.kind == kindParam:
		label += " (param)"
	case r.input.secret:
		label += " (secret)"
	}
	style, marker := th.label, markerOff
	if focused {
		style, marker = th.title, th.marker.Render(markerOn)
	}
	s := line(marker + style.Render(padRight(label, labelWidth)) + " " + value)
	if r.err != "" {
		s += line(markerOff + th.err.Render(r.err))
	}
	return s
}
