package search

import (
	"cmp"
	"slices"
	"strings"
	"time"
	"uuid"

	"github.com/elliot40404/creds/internal/vault"
	"github.com/sahilm/fuzzy"
)

type Summary struct {
	ID       uuid.UUID
	Path     string
	Name     string
	Type     vault.Type
	Host     string
	Username string
	Engine   string
	Tags     []string
	Created  time.Time
	Updated  time.Time
}

func summarize(e vault.Entry) Summary {
	engine, _ := e.Field(vault.FieldEngine)
	return Summary{
		ID:       e.ID,
		Path:     e.Path,
		Name:     e.Name,
		Type:     e.Type,
		Host:     e.Host,
		Username: e.Username,
		Engine:   engine,
		Tags:     slices.Clone(e.Tags),
		Created:  e.Created,
		Updated:  e.Updated,
	}
}

func SummarizeAll(entries []vault.Entry) []Summary {
	out := make([]Summary, len(entries))
	for i, e := range entries {
		out[i] = summarize(e)
	}
	return out
}

type Index struct {
	items []Summary
	keys  []string
}

func NewIndex(items []Summary) Index {
	sorted := slices.Clone(items)
	slices.SortFunc(sorted, byPath)
	keys := make([]string, len(sorted))
	for i, s := range sorted {
		keys[i] = strings.Join(append([]string{s.Path, s.Name, string(s.Type), s.Host, s.Username}, s.Tags...), " ")
	}
	return Index{items: sorted, keys: keys}
}

func (x Index) Len() int { return len(x.items) }

func (x Index) Paths() []string { return Paths(x.items) }

func SearchSort(items []Summary, query string, order Sort) []Summary {
	return NewIndex(items).Search(query, order)
}

func (x Index) Search(query string, order Sort) []Summary {
	out := x.search(query)
	if order != SortPath && order.Valid() {
		slices.SortStableFunc(out, byOrder(order))
	}
	return out
}

func (x Index) search(query string) []Summary {
	f := parseFilter(query)
	picked := x.pick(f)
	text := strings.TrimSpace(f.Text)
	if text == "" {
		out := make([]Summary, len(picked))
		for i, at := range picked {
			out[i] = x.items[at]
		}
		return out
	}
	matches := fuzzy.FindFromNoSort(text, subset{x.keys, picked})
	slices.SortStableFunc(matches, func(a, b fuzzy.Match) int { return cmp.Compare(b.Score, a.Score) })
	out := make([]Summary, len(matches))
	for i, m := range matches {
		out[i] = x.items[picked[m.Index]]
	}
	return out
}

func (x Index) pick(f filter) []int {
	out := make([]int, 0, len(x.items))
	for i, s := range x.items {
		if f.match(s) {
			out = append(out, i)
		}
	}
	return out
}

type subset struct {
	keys []string
	at   []int
}

func (s subset) Len() int { return len(s.at) }

func (s subset) String(i int) string { return s.keys[s.at[i]] }

func byOrder(order Sort) func(a, b Summary) int {
	when := func(s Summary) time.Time { return s.Updated }
	if order == SortCreated {
		when = func(s Summary) time.Time { return s.Created }
	}
	return func(a, b Summary) int {
		if c := when(b).Compare(when(a)); c != 0 {
			return c
		}
		return byPath(a, b)
	}
}

func byPath(a, b Summary) int {
	return cmp.Compare(a.Path, b.Path)
}
