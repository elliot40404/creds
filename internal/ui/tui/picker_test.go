package tui

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/elliot40404/creds/internal/app"
	"github.com/elliot40404/creds/internal/search"
	"github.com/elliot40404/creds/internal/vault"
)

func summaries(paths ...string) []search.Summary {
	var entries []vault.Entry
	for _, p := range paths {
		entries = append(entries, vault.Entry{Path: p, Type: vault.TypeLogin, Username: "me"})
	}
	return search.SummarizeAll(entries)
}

func TestListFilterCapturesText(t *testing.T) {
	l := newList(summaries("work/db", "home/wifi", "work/github"))
	s, msg := keys(l, append([]tea.KeyPressMsg{ch('/')}, text("qwifi")...)...)
	got := s.(listScreen)
	if msg != nil || got.query != "qwifi" || !got.typing() {
		t.Fatalf("query %q msg %v", got.query, msg)
	}
	s, _ = keys(s, back, back, back, back, back, ch('w'), ch('i'), ch('f'), enter)
	got = s.(listScreen)
	if got.typing() || len(got.shown) != 1 || got.current() != "home/wifi" {
		t.Fatalf("shown %v", got.shown)
	}
	s, msg = keys(s, esc)
	if s.(listScreen).query != "" || msg != nil {
		t.Fatalf("esc should clear filter, msg %v", msg)
	}
}

func TestListKeys(t *testing.T) {
	l := newList(summaries("a", "b", "c"))
	cases := []struct {
		keys []tea.KeyPressMsg
		want tea.Msg
	}{
		{[]tea.KeyPressMsg{down, enter}, openMsg{"b"}},
		{[]tea.KeyPressMsg{ch('j'), ch('j'), ch('j'), ch('k'), ch('y')}, copyMsg{path: "b"}},
		{[]tea.KeyPressMsg{up, ch('d')}, deleteMsg{"a"}},
		{[]tea.KeyPressMsg{ch('a')}, addMsg{}},
		{[]tea.KeyPressMsg{ch('s')}, syncMsg{}},
		{[]tea.KeyPressMsg{ch('L')}, lockMsg{}},
		{[]tea.KeyPressMsg{ch('q')}, quitMsg{}},
		{[]tea.KeyPressMsg{esc}, quitMsg{}},
	}
	for _, c := range cases {
		if _, got := keys(l, c.keys...); got != c.want {
			t.Fatalf("keys %v: got %#v want %#v", c.keys, got, c.want)
		}
	}
}

func TestListEmptyIgnoresEntryKeys(t *testing.T) {
	l := newList(nil)
	for _, k := range []tea.KeyPressMsg{enter, ch('y'), ch('d')} {
		if _, msg := keys(l, k); msg != nil {
			t.Fatalf("%v: %v", k, msg)
		}
	}
	if !strings.Contains(l.View(80, 10), "vault is empty") {
		t.Fatal("empty hint missing")
	}
}

func TestListScrollKeepsCursorVisible(t *testing.T) {
	var s Screen = newList(summaries("a", "b", "c", "d", "e"))
	s, _ = s.Update(tea.WindowSizeMsg{Width: 80, Height: listChrome + 2})
	s, _ = keys(s, down, down, down)
	view := ansi.Strip(s.View(80, listChrome+2))
	if !strings.Contains(view, "d  login") || strings.Contains(view, "a  login") {
		t.Fatalf("view %q", view)
	}
	s, _ = keys(s, up, up)
	if view = ansi.Strip(s.View(80, listChrome+2)); !strings.Contains(view, "b  login") {
		t.Fatalf("view %q", view)
	}
}

func TestListKeepsCursorOnReload(t *testing.T) {
	s, _ := keys(newList(summaries("a", "b", "c")), down, down)
	l := s.(listScreen).setItems(summaries("a", "c"))
	if l.current() != "c" {
		t.Fatalf("current %q", l.current())
	}
}

func listWith(items ...search.Summary) listScreen {
	return newList(items)
}

func sortedItems() []search.Summary {
	at := func(d int) time.Time { return time.Date(2026, 1, d, 0, 0, 0, 0, time.UTC) }
	return []search.Summary{
		{Path: "a/login", Type: vault.TypeLogin, Created: at(1), Updated: at(3)},
		{Path: "b/db", Type: vault.TypeDatabase, Engine: "postgres", Created: at(3), Updated: at(1)},
	}
}

func TestListTypeKeyCyclesTheQueryToken(t *testing.T) {
	l := listWith(sortedItems()...)
	types := app.Types()
	for _, want := range types {
		s, _ := l.Update(ch('t'))
		l = s.(listScreen)
		if got := search.TokenValue(l.query, search.KeyType); got != string(want) {
			t.Fatalf("query %q, want type:%s", l.query, want)
		}
	}
	s, _ := l.Update(ch('t'))
	l = s.(listScreen)
	if l.query != "" {
		t.Fatalf("query %q, want the token gone after the last type", l.query)
	}
}

func TestListTypeKeyKeepsTypedText(t *testing.T) {
	l := listWith(sortedItems()...)
	l.query = "prod"
	l.filter()
	s, _ := l.Update(ch('t'))
	l = s.(listScreen)
	if !strings.Contains(l.query, "prod") {
		t.Fatalf("query %q dropped the typed text", l.query)
	}
	if search.TokenValue(l.query, search.KeyType) == "" {
		t.Fatalf("query %q has no type token", l.query)
	}
}

func TestListTypeKeyFilters(t *testing.T) {
	l := listWith(sortedItems()...)
	l.query = "type:database"
	l.filter()
	if len(l.shown) != 1 || l.shown[0].Path != "b/db" {
		t.Fatalf("shown %d", len(l.shown))
	}
}

func TestListOrderKeyCyclesSort(t *testing.T) {
	l := listWith(sortedItems()...)
	if l.shown[0].Path != "a/login" || strings.Contains(l.counter(), "sort") {
		t.Fatalf("start %q %q", l.shown[0].Path, l.counter())
	}
	s, _ := l.Update(ch('o'))
	l = s.(listScreen)
	if l.sort != search.SortUpdated || l.shown[0].Path != "a/login" {
		t.Fatalf("updated %v %q", l.sort, l.shown[0].Path)
	}
	if !strings.Contains(l.counter(), "sort updated") {
		t.Fatalf("counter %q", l.counter())
	}
	s, _ = l.Update(ch('o'))
	l = s.(listScreen)
	if l.sort != search.SortCreated || l.shown[0].Path != "b/db" {
		t.Fatalf("created %v %q", l.sort, l.shown[0].Path)
	}
	s, _ = l.Update(ch('o'))
	l = s.(listScreen)
	if l.sort != search.SortPath || strings.Contains(l.counter(), "sort") {
		t.Fatalf("back to path %v %q", l.sort, l.counter())
	}
}

func TestListKeysDoNotCollide(t *testing.T) {
	seen := map[string]string{}
	for _, k := range listWith(sortedItems()...).keys() {
		for part := range strings.SplitSeq(k.key, " ") {
			if was, dup := seen[part]; dup {
				t.Fatalf("key %q is both %q and %q", part, was, k.desc)
			}
			seen[part] = k.desc
		}
	}
}

func matchRuns(out string) int {
	on := th.match.Render("x")
	return strings.Count(out, on[:strings.Index(on, "x")])
}

func TestHighlightUsesTheFilterTextAndWholeRuns(t *testing.T) {
	cols := columns{path: 20, typ: 8}
	row := search.Summary{Path: "work/db", Type: vault.TypeDatabase, Username: "carol", Host: "h"}
	if n := matchRuns(cols.describe(th, row, "tag:work db")); n != 1 {
		t.Fatalf("path runs %d, want 1", n)
	}
	if n := matchRuns(cols.describe(th, row, "carol")); n != 1 {
		t.Fatalf("owner runs %d, want 1", n)
	}
}
