package tui

import (
	"maps"
	"slices"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/elliot40404/creds/internal/render"
	"github.com/elliot40404/creds/internal/vault"
)

const masked = "********"

type field struct {
	name   string
	value  string
	secret bool
}

type detailScreen struct {
	list  fieldList
	entry vault.Entry
}

func newDetail(e vault.Entry) detailScreen {
	return detailScreen{entry: e, list: fieldList{fields: fieldsOf(e)}}
}

func fieldsOf(e vault.Entry) []field {
	var out []field
	for _, f := range []field{{"username", e.Username, false}, {"host", e.Host, false}, {"url", e.URL, false}} {
		if f.value != "" {
			out = append(out, f)
		}
	}
	for _, f := range e.Fields {
		out = append(out, field{f.Name, f.Value, f.Secret})
	}
	for _, k := range slices.Sorted(maps.Keys(e.Params)) {
		out = append(out, field{k, e.Params[k], render.SecretParam(k)})
	}
	if e.Notes != "" {
		out = append(out, field{"notes", e.Notes, false})
	}
	return out
}

func (d detailScreen) keys() []keyHelp {
	keys := []keyHelp{
		{"c", "copy field"},
		{"r", "reveal field"},
		{"esc", "back"},
		{"y", "copy default"},
		{"e", "edit"},
		{"d", "delete"},
	}
	if d.entry.Type == vault.TypeDatabase {
		keys = append(keys, keyHelp{"f", "copy as format"})
	}
	if len(d.entry.History) > 0 {
		keys = append(keys, keyHelp{"h", "previous versions"})
	}
	return append(keys, keyHelp{"up/down j/k", "move"}, keyHelp{"s", "sync now"}, keyHelp{"L", "lock and quit"})
}

func (d detailScreen) Update(msg tea.Msg) (Screen, tea.Cmd) {
	if r, ok := msg.(revealedMsg); ok {
		return d.reveal(r.entry), nil
	}
	key, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return d, nil
	}
	path := d.entry.Path
	if dy := navDelta(key); dy != 0 {
		d.list = d.list.move(dy)
		return d, nil
	}
	switch key.String() {
	case "r":
		var cmd tea.Cmd
		d.list, cmd = d.list.toggle(path)
		return d, cmd
	case "c":
		if len(d.list.fields) > 0 {
			return d, send(copyMsg{path: path, field: d.list.fields[d.list.cursor].name})
		}
	case "y":
		return d, send(copyMsg{path: path})
	case "f":
		if d.entry.Type == vault.TypeDatabase {
			return d, send(formatsMsg{path})
		}
	case "h":
		if len(d.entry.History) > 0 {
			return d, send(historyMsg{d.entry})
		}
	case "e":
		return d, send(editMsg{path})
	case "d":
		return d, send(deleteMsg{path})
	case "s":
		return d, send(syncMsg{})
	case "L":
		return d, send(lockMsg{})
	case "esc", "q":
		return d, send(popMsg{})
	}
	return d, nil
}

func (d detailScreen) reveal(e vault.Entry) detailScreen {
	if e.Path != d.entry.Path {
		return d
	}
	d.entry, d.list.fields = e, fieldsOf(e)
	d.list.cursor = min(d.list.cursor, max(len(d.list.fields)-1, 0))
	d.list.revealed = len(d.list.fields) > 0
	return d
}

func (d detailScreen) View(width, height int) string {
	var b strings.Builder
	b.WriteString(line(th.title.Render(d.entry.Path) + "  " + th.badge.Render(string(d.entry.Type))))
	if len(d.entry.Tags) > 0 {
		b.WriteString(line(th.dim.Render("tags " + strings.Join(d.entry.Tags, ", "))))
	}
	b.WriteString(line(th.dim.Render(metaLine(d.entry))))
	b.WriteString(line(""))
	if len(d.list.fields) == 0 {
		b.WriteString(line(th.dim.Render("no fields, press e to edit")))
	}
	b.WriteString(d.list.view(width, height-4))
	return b.String()
}

func fieldRow(t theme, f field, w int, shown bool) string {
	label := t.label.Render(padRight(f.name, w) + "  ")
	value := f.value
	switch {
	case f.secret && !shown:
		return label + t.masked.Render(masked)
	case strings.Contains(value, "\n"):
		first, _, _ := strings.Cut(value, "\n")
		value = first + " ..."
	}
	return label + t.text.Render(value)
}

func widestName(fields []field) int {
	return min(widest(fields, func(f field) string { return f.name }), labelWidth)
}

func stamp(t time.Time) string {
	if t.IsZero() {
		return "never"
	}
	return t.Local().Format("2006-01-02 15:04")
}

func metaLine(e vault.Entry) string {
	parts := []string{"created " + stamp(e.Created), "updated " + stamp(e.Updated)}
	if e.Machine != "" {
		parts = append(parts, "on "+e.Machine)
	}
	return strings.Join(parts, "  ")
}
