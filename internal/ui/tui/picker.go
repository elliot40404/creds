package tui

import (
	"fmt"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/elliot40404/creds/internal/app"
	"github.com/elliot40404/creds/internal/search"
	"github.com/elliot40404/creds/internal/vault"
	"github.com/sahilm/fuzzy"
)

const listChrome = 2

type listScreen struct {
	index     search.Index
	shown     []search.Summary
	query     string
	filtering bool
	cursor    int
	offset    int
	rows      int
	sort      search.Sort
}

func newList(items []search.Summary) listScreen {
	l := listScreen{index: search.NewIndex(items), rows: 10, sort: search.SortPath}
	l.filter()
	return l
}

func (l listScreen) setItems(items []search.Summary) listScreen {
	path := l.current()
	l.index = search.NewIndex(items)
	l.filter()
	for i, s := range l.shown {
		if s.Path == path {
			l.cursor = i
		}
	}
	l.scroll()
	return l
}

func (l listScreen) current() string {
	if len(l.shown) == 0 {
		return ""
	}
	return l.shown[l.cursor].Path
}

func (l listScreen) typing() bool {
	return l.filtering
}

func (l listScreen) keys() []keyHelp {
	if l.filtering {
		return []keyHelp{{"enter", "keep filter"}, {"esc", "clear filter"}, {"up/down", "move"}}
	}
	return []keyHelp{
		{"enter", "open"},
		{"y", "copy default"},
		{"/", "filter, try type:database engine:postgres tag:work"},
		{"t", "cycle the type filter"},
		{"o", "cycle the order"},
		{"a", "add"},
		{"d", "delete"},
		{"s", "sync now"},
		{"L", "lock and quit"},
		{"up/down j/k", "move"},
		{"q esc", "quit"},
		{",", "settings"},
	}
}

func (l listScreen) Update(msg tea.Msg) (Screen, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		l.rows = max(msg.Height-listChrome, 1)
		l.scroll()
	case tea.KeyPressMsg:
		if l.filtering {
			return l.filterKey(msg)
		}
		return l.navKey(msg)
	}
	return l, nil
}

func (l listScreen) filterKey(key tea.KeyPressMsg) (Screen, tea.Cmd) {
	switch key.String() {
	case "esc":
		l.query, l.filtering = "", false
		l.filter()
	case "enter":
		l.filtering = false
	case "up", "down", "ctrl+p", "ctrl+n":
		l.filtering = false
		return l.navKey(key)
	case "backspace":
		if r := []rune(l.query); len(r) > 0 {
			l.query = string(r[:len(r)-1])
			l.filter()
		}
	default:
		if key.Text != "" {
			l.query += key.Text
			l.filter()
		}
	}
	return l, nil
}

func (l listScreen) navKey(key tea.KeyPressMsg) (Screen, tea.Cmd) {
	switch key.String() {
	case "/":
		l.filtering = true
	case "up", "k", "ctrl+p":
		l.move(-1)
	case "down", "j", "ctrl+n":
		l.move(1)
	case "q":
		return l, send(quitMsg{})
	case "esc":
		if l.query == "" {
			return l, send(quitMsg{})
		}
		l.query = ""
		l.filter()
	case "a":
		return l, send(addMsg{})
	case "s":
		return l, send(syncMsg{})
	case "t":
		l.cycleType()
	case "o":
		l.sort = l.sort.Next()
		l.filter()
	case ",":
		return l, send(settingsMsg{})
	case "L":
		return l, send(lockMsg{})
	default:
		return l, l.entryKey(key.String())
	}
	return l, nil
}

func (l listScreen) entryKey(key string) tea.Cmd {
	path := l.current()
	if path == "" {
		return nil
	}
	switch key {
	case "enter":
		return send(openMsg{path})
	case "y":
		return send(copyMsg{path: path})
	case "d":
		return send(deleteMsg{path})
	}
	return nil
}

func (l *listScreen) filter() {
	l.shown = l.index.Search(l.query, l.sort)
	l.cursor, l.offset = 0, 0
}

func (l *listScreen) cycleType() {
	types := app.Types()
	i := slices.IndexFunc(types, func(t vault.Type) bool {
		return strings.EqualFold(string(t), search.TokenValue(l.query, search.KeyType))
	})
	next := ""
	if i+1 < len(types) {
		next = string(types[i+1])
	}
	l.query = search.WithToken(l.query, search.KeyType, next)
	l.filter()
}

func (l *listScreen) move(delta int) {
	l.cursor = moveCursor(l.cursor, delta, len(l.shown))
	l.scroll()
}

func (l *listScreen) scroll() {
	l.offset = min(l.offset, l.cursor)
	if l.cursor >= l.offset+l.rows {
		l.offset = l.cursor - l.rows + 1
	}
}

func (l listScreen) View(width, _ int) string {
	var b strings.Builder
	b.WriteString(line(spread(width, l.prompt(), th.dim.Render(l.counter()))))
	b.WriteString(line(""))
	visible := l.shown[l.offset:min(l.offset+l.rows, len(l.shown))]
	cols := columnsOf(visible, width)
	for i, s := range visible {
		b.WriteString(pickRow(width, l.offset+i == l.cursor, func(t theme) string {
			return cols.describe(t, s, l.query)
		}))
	}
	if l.index.Len() == 0 {
		b.WriteString(line(th.dim.Render("vault is empty, press a to add or run creds add")))
	}
	return b.String()
}

func (l listScreen) prompt() string {
	if !l.filtering && l.query == "" {
		return th.dim.Render("/ to filter")
	}
	cursor := ""
	if l.filtering {
		cursor = th.marker.Render("_")
	}
	return th.key.Render("/ ") + th.title.Render(l.query) + cursor
}

type columns struct {
	path int
	typ  int
}

func columnsOf(items []search.Summary, width int) columns {
	return columns{
		path: min(widest(items, func(s search.Summary) string { return s.Path }), max(width*2/5, 1)),
		typ:  widest(items, func(s search.Summary) string { return string(s.Type) }),
	}
}

func (c columns) describe(t theme, s search.Summary, query string) string {
	path := s.Path
	if r := []rune(path); len(r) > c.path {
		path = string(r[:max(c.path-1, 0)]) + ellipsis
	}
	text := search.FilterText(query)
	shown, hit := highlight(t.text, t.match, path, text)
	out := shown + t.text.Render(strings.Repeat(" ", c.path-len([]rune(path))+2))
	out += t.badge.Render(padRight(string(s.Type), c.typ))
	if who := owner(s); who != "" {
		if hit {
			text = ""
		}
		shown, _ := highlight(t.dim, t.match, who, text)
		out += t.dim.Render("  ") + shown
	}
	return out
}

func owner(s search.Summary) string {
	switch {
	case s.Username != "" && s.Host != "":
		return s.Username + "@" + s.Host
	case s.Username != "":
		return s.Username
	}
	return s.Host
}

func highlight(base, match style, s, text string) (string, bool) {
	matches := fuzzy.Find(text, []string{s})
	if text == "" || len(matches) == 0 {
		return base.Render(s), false
	}
	hit := make(map[int]bool, len(matches[0].MatchedIndexes))
	for _, i := range matches[0].MatchedIndexes {
		hit[i] = true
	}
	var b, run strings.Builder
	on := false
	flush := func() {
		if run.Len() == 0 {
			return
		}
		st := base
		if on {
			st = match
		}
		b.WriteString(st.Render(run.String()))
		run.Reset()
	}
	for i, r := range s {
		if hit[i] != on {
			flush()
			on = hit[i]
		}
		run.WriteRune(r)
	}
	flush()
	return b.String(), true
}

func (l listScreen) counter() string {
	s := fmt.Sprintf("%d/%d", len(l.shown), l.index.Len())
	if l.sort != search.SortPath {
		s += "  sort " + string(l.sort)
	}
	return s
}
