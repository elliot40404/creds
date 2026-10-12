package search

import (
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/elliot40404/creds/internal/vault"
)

const secret = "hunter2-very-secret"

func entry(path string, t vault.Type) vault.Entry {
	return vault.Entry{
		Path:     path,
		Type:     t,
		Username: "admin",
		URL:      "postgres://admin:" + secret + "@db",
		Notes:    secret,
		Fields:   []vault.Field{{Name: "password", Value: secret, Secret: true}},
		Params:   map[string]string{"sslmode": secret},
	}
}

func TestSummaryHasNoSecrets(t *testing.T) {
	s := summarize(entry("work/db", vault.TypeDatabase))
	if got := fmt.Sprintf("%+v", s); strings.Contains(got, secret) {
		t.Fatal("summary leaks secret")
	}
	want := []string{"ID", "Path", "Name", "Type", "Host", "Username", "Engine", "Tags", "Created", "Updated", "Frecency"}
	var got []string
	for f := range reflect.TypeFor[Summary]().Fields() {
		got = append(got, f.Name)
	}
	if !slices.Equal(got, want) {
		t.Fatalf("fields = %v", got)
	}
}

func TestSummarizeClonesTags(t *testing.T) {
	e := entry("a", vault.TypeLogin)
	e.Tags = []string{"x"}
	s := summarize(e)
	e.Tags[0] = "y"
	if s.Tags[0] != "x" {
		t.Fatal("tags shared")
	}
}

func TestEmptyQuerySortedByPath(t *testing.T) {
	items := SummarizeAll([]vault.Entry{
		entry("zeta", vault.TypeNote),
		entry("alpha", vault.TypeNote),
		entry("mid", vault.TypeNote),
	})
	got := Paths(SearchSort(items, "  ", SortPath))
	if !slices.Equal(got, []string{"alpha", "mid", "zeta"}) {
		t.Fatalf("got %v", got)
	}
	if items[0].Path != "zeta" {
		t.Fatal("input mutated")
	}
}

func TestRanking(t *testing.T) {
	items := SummarizeAll([]vault.Entry{
		entry("personal/github", vault.TypeLogin),
		entry("work/gitlab", vault.TypeLogin),
		entry("work/prod/db", vault.TypeDatabase),
		entry("home/wifi", vault.TypeNote),
	})
	got := Paths(SearchSort(items, "wpd", SortPath))
	if len(got) == 0 || got[0] != "work/prod/db" {
		t.Fatalf("got %v", got)
	}
	got = Paths(SearchSort(items, "git", SortPath))
	slices.Sort(got)
	if !slices.Equal(got, []string{"personal/github", "work/gitlab"}) {
		t.Fatalf("got %v", got)
	}
	if got := SearchSort(items, "qqq", SortPath); len(got) != 0 {
		t.Fatalf("got %v", Paths(got))
	}
}

func TestMatchesTagsAndHost(t *testing.T) {
	a := entry("a", vault.TypeLogin)
	a.Tags = []string{"billing"}
	b := entry("b", vault.TypeLogin)
	b.Host = "db.example.com"
	items := SummarizeAll([]vault.Entry{a, b})
	if got := Paths(SearchSort(items, "billing", SortPath)); !slices.Equal(got, []string{"a"}) {
		t.Fatalf("tag got %v", got)
	}
	if got := Paths(SearchSort(items, "example", SortPath)); !slices.Equal(got, []string{"b"}) {
		t.Fatalf("host got %v", got)
	}
}

func TestSecretNotSearchable(t *testing.T) {
	items := SummarizeAll([]vault.Entry{entry("a", vault.TypeLogin)})
	if got := SearchSort(items, "hunter2", SortPath); len(got) != 0 {
		t.Fatal("secret matched")
	}
}

func TestTieBrokenByPath(t *testing.T) {
	items := SummarizeAll([]vault.Entry{entry("b/x", vault.TypeNote), entry("a/x", vault.TypeNote)})
	if got := Paths(SearchSort(items, "x", SortPath)); !slices.Equal(got, []string{"a/x", "b/x"}) {
		t.Fatalf("got %v", got)
	}
}

func TestWordsMatchInAnyOrder(t *testing.T) {
	items := SummarizeAll([]vault.Entry{entry("work/db", vault.TypeDatabase), entry("home/wifi", vault.TypeNote)})
	for _, q := range []string{"work db", "db work", "  db   work "} {
		if got := pathLine(SearchSort(items, q, SortPath)); got != "work/db" {
			t.Fatalf("%q got %q", q, got)
		}
	}
	if got := SearchSort(items, "db wifi", SortPath); len(got) != 0 {
		t.Fatalf("every word must match, got %v", Paths(got))
	}
}

func TestSmartCase(t *testing.T) {
	items := SummarizeAll([]vault.Entry{entry("work/DB", vault.TypeNote), entry("work/db", vault.TypeNote)})
	cases := map[string]string{
		"db": "work/DB work/db",
		"DB": "work/DB",
		"Db": "",
	}
	for q, want := range cases {
		if got := pathLine(SearchSort(items, q, SortPath)); got != want {
			t.Fatalf("%q got %q, want %q", q, got, want)
		}
	}
}

func TestAccentsFolded(t *testing.T) {
	items := SummarizeAll([]vault.Entry{entry("perso/café", vault.TypeNote)})
	if got := pathLine(SearchSort(items, "cafe", SortPath)); got != "perso/café" {
		t.Fatalf("got %q", got)
	}
}

func TestWordBoundaryRanksFirst(t *testing.T) {
	items := SummarizeAll([]vault.Entry{entry("dashboard", vault.TypeNote), entry("work/db", vault.TypeNote)})
	if got := Paths(SearchSort(items, "db", SortPath)); got[0] != "work/db" {
		t.Fatalf("got %v", got)
	}
}

func TestOperators(t *testing.T) {
	items := SummarizeAll([]vault.Entry{
		entry("work/db", vault.TypeDatabase),
		entry("work/old/db", vault.TypeDatabase),
		entry("home/dashboard", vault.TypeNote),
		entry("home/!bang", vault.TypeNote),
	})
	cases := map[string]string{
		"exclude":            "db !old",
		"exclude only":       "!work",
		"exact":              "'db",
		"starts with":        "^work",
		"ends with":          "db$",
		"whole field":        "^work/db$",
		"exclude prefix":     "^work !^work/old",
		"bang alone is text": "!",
		"quote alone":        "'",
		"fuzzy needs no op":  "hdb",
	}
	want := map[string]string{
		"exclude":            "work/db home/dashboard",
		"exclude only":       "home/!bang home/dashboard",
		"exact":              "work/db work/old/db",
		"starts with":        "work/db work/old/db",
		"ends with":          "work/db work/old/db",
		"whole field":        "work/db",
		"exclude prefix":     "work/db",
		"bang alone is text": "home/!bang",
		"quote alone":        "",
		"fuzzy needs no op":  "home/dashboard",
	}
	for name, q := range cases {
		if got := pathLine(SearchSort(items, q, SortPath)); got != want[name] {
			t.Errorf("%s %q got %q, want %q", name, q, got, want[name])
		}
	}
}

func TestNameHitBeatsHostHit(t *testing.T) {
	a := entry("aaa", vault.TypeLogin)
	a.Host = "github.com"
	b := entry("zzz", vault.TypeLogin)
	b.Name = "github"
	got := Paths(SearchSort(SummarizeAll([]vault.Entry{a, b}), "github", SortPath))
	if !slices.Equal(got, []string{"zzz", "aaa"}) {
		t.Fatalf("got %v", got)
	}
}

func TestPositions(t *testing.T) {
	cases := []struct {
		query, s string
		want     []int
	}{
		{"db work", "work/db", []int{0, 1, 2, 3, 5, 6}},
		{"!work db", "work/db", []int{5, 6}},
		{"tag:x db", "work/db", []int{5, 6}},
		{"fé", "café", []int{2, 3}},
		{"", "work/db", nil},
		{"zz", "work/db", nil},
	}
	for _, tc := range cases {
		if got := Positions(tc.query, tc.s); !slices.Equal(got, tc.want) {
			t.Errorf("%q in %q got %v, want %v", tc.query, tc.s, got, tc.want)
		}
	}
}

func frecentItems() []Summary {
	return []Summary{
		{Path: "a/db", Frecency: 0},
		{Path: "b/db", Frecency: 6},
		{Path: "c/db", Frecency: 1},
		{Path: "d/dashboard", Frecency: 50},
		{Path: "e/xdxxxxxxxxb", Frecency: 50},
	}
}

func TestFrecentEmptyQueryMostUsedFirst(t *testing.T) {
	if got := pathLine(SearchSort(frecentItems(), "", SortFrecent)); got != "d/dashboard e/xdxxxxxxxxb b/db c/db a/db" {
		t.Fatalf("got %q", got)
	}
	if got := pathLine(SearchSort(frecentItems(), "", SortPath)); got != "a/db b/db c/db d/dashboard e/xdxxxxxxxxb" {
		t.Fatalf("path sort got %q", got)
	}
}

func TestFrecentBoostsCloseMatches(t *testing.T) {
	got := Paths(SearchSort(frecentItems(), "db", SortFrecent))
	if !slices.Equal(got, []string{"b/db", "c/db", "d/dashboard", "a/db", "e/xdxxxxxxxxb"}) {
		t.Fatalf("got %v, want use to break close calls only", got)
	}
	if got := Paths(SearchSort(frecentItems(), "db", SortPath)); got[0] != "a/db" {
		t.Fatalf("path sort used frecency: %v", got)
	}
}

func TestBoostIsCapped(t *testing.T) {
	if boost(0) != 0 || boost(-1) != 0 || boost(1e9) > maxBoost || boost(2) != maxBoost/2 {
		t.Fatal("boost out of range")
	}
}
