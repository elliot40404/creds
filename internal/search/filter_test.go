package search

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/elliot40404/creds/internal/vault"
)

func TestParseFilter(t *testing.T) {
	cases := map[string]struct {
		query string
		want  filter
	}{
		"plain text": {"web mail", filter{Text: "web mail"}},
		"one key":    {"type:database", filter{Types: []string{"database"}}},
		"all keys": {"type:database engine:postgres tag:work", filter{
			Types: []string{"database"}, Engines: []string{"postgres"}, Tags: []string{"work"},
		}},
		"keys and text": {"type:login prod box", filter{
			Types: []string{"login"}, Text: "prod box",
		}},
		"key at the end": {"prod type:login", filter{
			Types: []string{"login"}, Text: "prod",
		}},
		"repeated key": {"tag:a tag:b", filter{Tags: []string{"a", "b"}}},
		"unknown key":  {"foo:bar", filter{Text: "foo:bar"}},
		"url stays":    {"https://example.com", filter{Text: "https://example.com"}},
		"empty value":  {"type:", filter{Text: "type:"}},
		"upper key":    {"TYPE:login", filter{Types: []string{"login"}}},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if got := parseFilter(tc.query); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("got %+v, want %+v", got, tc.want)
			}
		})
	}
}

func filterItems() []Summary {
	return []Summary{
		{Path: "work/db/pg", Type: vault.TypeDatabase, Engine: "postgres", Tags: []string{"work", "prod"}},
		{Path: "work/db/mongo", Type: vault.TypeDatabase, Engine: "mongo", Tags: []string{"work"}},
		{Path: "web/mail", Type: vault.TypeLogin, Tags: []string{"personal"}},
		{Path: "web/bank", Type: vault.TypeLogin, Tags: []string{"prod"}},
	}
}

func pathLine(items []Summary) string {
	return strings.Join(Paths(items), " ")
}

func TestSearchFilters(t *testing.T) {
	cases := map[string]struct {
		query string
		want  string
	}{
		"no query":        {"", "web/bank web/mail work/db/mongo work/db/pg"},
		"type":            {"type:database", "work/db/mongo work/db/pg"},
		"engine":          {"engine:postgres", "work/db/pg"},
		"type and engine": {"type:database engine:mongo", "work/db/mongo"},
		"types or":        {"type:database type:login", "web/bank web/mail work/db/mongo work/db/pg"},
		"tags or":         {"tag:personal tag:prod", "web/bank web/mail work/db/pg"},
		"and across":      {"type:login tag:prod", "web/bank"},
		"case folded":     {"TYPE:DATABASE engine:POSTGRES", "work/db/pg"},
		"exact not fuzzy": {"type:DB", ""},
		"with text":       {"type:database mongo", "work/db/mongo"},
		"no match":        {"engine:mysql", ""},
		"unknown key":     {"foo:bar", ""},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if got := pathLine(SearchSort(filterItems(), tc.query, SortPath)); got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestSearchKeepsFuzzyTextWithAFilter(t *testing.T) {
	got := pathLine(SearchSort(filterItems(), "type:database wdp", SortPath))
	if got != "work/db/pg" {
		t.Fatalf("got %q", got)
	}
}

func timedItems() []Summary {
	at := func(d int) time.Time { return time.Date(2026, 1, d, 0, 0, 0, 0, time.UTC) }
	return []Summary{
		{Path: "b", Created: at(1), Updated: at(3)},
		{Path: "a", Created: at(2), Updated: at(3)},
		{Path: "c", Created: at(3), Updated: at(1)},
	}
}

func TestSearchSort(t *testing.T) {
	cases := map[string]struct {
		order Sort
		want  string
	}{
		"path":               {SortPath, "a b c"},
		"updated newest":     {SortUpdated, "a b c"},
		"created newest":     {SortCreated, "c a b"},
		"unknown falls back": {Sort("nope"), "a b c"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if got := pathLine(SearchSort(timedItems(), "", tc.order)); got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestSortTieBreaksOnPath(t *testing.T) {
	got := pathLine(SearchSort(timedItems(), "", SortUpdated))
	if got != "a b c" {
		t.Fatalf("got %q, want the two equal updated times ordered by path", got)
	}
}

func TestSortWinsOverRelevance(t *testing.T) {
	items := timedItems()
	byRelevance := pathLine(SearchSort(items, "", SortPath))
	byCreated := pathLine(SearchSort(items, "", SortCreated))
	if byRelevance == byCreated {
		t.Fatal("sort made no difference")
	}
}

func TestSortNextCycles(t *testing.T) {
	got := DefaultSort
	var seen []string
	for range len(Sorts()) {
		got = got.Next()
		seen = append(seen, string(got))
	}
	if strings.Join(seen, " ") != "path updated created frecent" {
		t.Fatalf("cycle %q", seen)
	}
	if Sort("nope").Next() != DefaultSort {
		t.Fatal("unknown sort does not reset")
	}
}

func TestHostUserFolderFilters(t *testing.T) {
	items := []Summary{
		{Path: "work/db/pg", Host: "pg.Example.com", Username: "admin"},
		{Path: "work/mail", Host: "mail.example.com", Username: "me"},
		{Path: "Home/wifi", Username: "Admin2"},
	}
	cases := map[string]string{
		"host part":       "host:example",
		"host case":       "host:EXAMPLE.com",
		"host or":         "host:pg host:nope",
		"user part":       "user:admin",
		"folder":          "in:work/",
		"folder case":     "in:home",
		"folder and text": "in:work/ mail",
		"folder no mid":   "in:db",
	}
	want := map[string]string{
		"host part":       "work/db/pg work/mail",
		"host case":       "work/db/pg work/mail",
		"host or":         "work/db/pg",
		"user part":       "Home/wifi work/db/pg",
		"folder":          "work/db/pg work/mail",
		"folder case":     "Home/wifi",
		"folder and text": "work/mail",
		"folder no mid":   "",
	}
	for name, q := range cases {
		if got := pathLine(SearchSort(items, q, SortPath)); got != want[name] {
			t.Errorf("%s %q got %q, want %q", name, q, got, want[name])
		}
	}
}

func TestParseNewKeys(t *testing.T) {
	got := parseFilter("host:a user:b in:c/ x")
	want := filter{Hosts: []string{"a"}, Users: []string{"b"}, Folders: []string{"c/"}, Text: "x"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v", got)
	}
}
