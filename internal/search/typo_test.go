package search

import (
	"testing"

	"github.com/elliot40404/creds/internal/vault"
)

func TestNear(t *testing.T) {
	cases := []struct {
		a, b string
		k    int
		want bool
	}{
		{"postgers", "postgres", 1, true},
		{"pord", "prod", 1, true},
		{"pord", "production", 1, true},
		{"prdo", "prod", 1, true},
		{"gitbub", "github", 1, true},
		{"gihub", "github", 1, true},
		{"githubb", "github", 1, true},
		{"mongo", "mysql", 1, false},
		{"abcd", "wxyz", 1, false},
		{"databse", "database", 1, true},
		{"dtaabse", "database", 2, true},
		{"dtaabse", "database", 1, false},
	}
	for _, tc := range cases {
		if got := near([]rune(tc.a), []rune(tc.b), tc.k); got != tc.want {
			t.Errorf("near(%q, %q, %d) = %v", tc.a, tc.b, tc.k, got)
		}
	}
}

func TestTypoSearch(t *testing.T) {
	db := entry("work/postgres", vault.TypeDatabase)
	tagged := entry("home/router", vault.TypeLogin)
	tagged.Tags = []string{"production"}
	items := SummarizeAll([]vault.Entry{db, tagged, entry("misc/abc", vault.TypeNote)})
	cases := map[string]string{
		"postgers":     "work/postgres",
		"pord":         "home/router",
		"POSTGERS":     "",
		"abd":          "",
		"!postgers":    "home/router misc/abc work/postgres",
		"'postgers":    "",
		"routr postgr": "",
	}
	for q, want := range cases {
		if got := pathLine(SearchSort(items, q, SortPath)); got != want {
			t.Errorf("%q got %q, want %q", q, got, want)
		}
	}
}

func TestTyposOnlyWhenNothingMatches(t *testing.T) {
	items := SummarizeAll([]vault.Entry{
		entry("a/prdo", vault.TypeNote),
		entry("b/pxrxoxd", vault.TypeNote),
	})
	if got := pathLine(SearchSort(items, "prod", SortPath)); got != "b/pxrxoxd" {
		t.Fatalf("real hit got %q", got)
	}
	if got := pathLine(SearchSort(items, "pord", SortPath)); got != "a/prdo" {
		t.Fatalf("typo got %q", got)
	}
}

func TestFewerTyposFirst(t *testing.T) {
	items := SummarizeAll([]vault.Entry{
		entry("wrko/postgers", vault.TypeNote),
		entry("work/postgers", vault.TypeNote),
	})
	if got := pathLine(SearchSort(items, "work postgres", SortPath)); got != "work/postgers wrko/postgers" {
		t.Fatalf("got %q", got)
	}
}

func TestTypoPositions(t *testing.T) {
	got := Positions("postgers", "work/postgres")
	if len(got) != 8 || got[0] != 5 {
		t.Fatalf("got %v", got)
	}
}
