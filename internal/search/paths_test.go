package search

import (
	"reflect"
	"testing"
)

func TestPathsFromSummaries(t *testing.T) {
	items := []Summary{{Path: "b"}, {Path: "a"}}
	if got := Paths(items); !reflect.DeepEqual(got, []string{"b", "a"}) {
		t.Fatalf("got %q", got)
	}
}

func TestPrefixes(t *testing.T) {
	cases := map[string]struct {
		in   []string
		want []string
	}{
		"none":        {[]string{"alpha", "beta"}, nil},
		"empty":       {nil, nil},
		"one level":   {[]string{"work/db"}, []string{"work/"}},
		"nested":      {[]string{"work/db/pg"}, []string{"work/", "work/db/"}},
		"shared":      {[]string{"work/db/pg", "work/db/redis", "work/mail"}, []string{"work/", "work/db/"}},
		"sorted":      {[]string{"z/a", "a/b"}, []string{"a/", "z/"}},
		"trailing":    {[]string{"work/"}, []string{"work/"}},
		"mixed depth": {[]string{"a", "a/b", "a/b/c"}, []string{"a/", "a/b/"}},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if got := Prefixes(tc.in); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestCandidates(t *testing.T) {
	got := Candidates([]string{"work/db/pg", "work/mail", "solo"})
	want := []string{"solo", "work/", "work/db/", "work/db/pg", "work/mail"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestComplete(t *testing.T) {
	cands := Candidates([]string{"work/db/pg", "work/db/redis", "web/mail"})
	cases := map[string]struct {
		typed   string
		filled  string
		matches int
	}{
		"empty":        {"", "", 0},
		"one branch":   {"wo", "work/", 4},
		"deeper":       {"work/", "work/db/", 3},
		"single match": {"work/db/p", "work/db/pg", 1},
		"exact leaf":   {"work/db/pg", "", 0},
		"no match":     {"zzz", "", 0},
		"two roots":    {"w", "w", 6},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			filled, matches := Complete(cands, tc.typed)
			if filled != tc.filled {
				t.Fatalf("filled %q, want %q", filled, tc.filled)
			}
			if tc.matches > 0 && len(matches) != tc.matches {
				t.Fatalf("%d matches %q, want %d", len(matches), matches, tc.matches)
			}
		})
	}
}

func TestCompleteStaysOnARuneBoundary(t *testing.T) {
	filled, _ := Complete([]string{"日本語/a", "日曜/b"}, "日")
	if filled != "日" {
		t.Fatalf("filled %q", filled)
	}
}
