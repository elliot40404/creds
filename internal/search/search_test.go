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
	want := []string{"ID", "Path", "Name", "Type", "Host", "Username", "Engine", "Tags", "Created", "Updated"}
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
