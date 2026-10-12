package search

import (
	"cmp"
	"iter"
	"slices"
	"time"
	"uuid"

	"github.com/elliot40404/creds/internal/vault"
	"github.com/junegunn/fzf/src/util"
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
	items  []Summary
	fields [][]util.Chars
}

func NewIndex(items []Summary) Index {
	sorted := slices.Clone(items)
	slices.SortFunc(sorted, byPath)
	size, count := 0, 0
	for _, s := range sorted {
		for f := range texts(s) {
			size += len(f)
			count++
		}
	}
	buf := make([]byte, 0, size)
	all := make([]util.Chars, 0, count)
	fields := make([][]util.Chars, len(sorted))
	for i, s := range sorted {
		from := len(all)
		for f := range texts(s) {
			at := len(buf)
			buf = append(buf, f...)
			all = append(all, util.ToChars(buf[at:len(buf):len(buf)]))
		}
		fields[i] = all[from:len(all):len(all)]
	}
	return Index{items: sorted, fields: fields}
}

func texts(s Summary) iter.Seq[string] {
	return func(yield func(string) bool) {
		for _, f := range [...]string{s.Path, s.Name, string(s.Type), s.Host, s.Username} {
			if f != "" && !yield(f) {
				return
			}
		}
		for _, f := range s.Tags {
			if f != "" && !yield(f) {
				return
			}
		}
	}
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
	words := terms(f.Text)
	if len(words) == 0 {
		out := make([]Summary, len(picked))
		for i, at := range picked {
			out[i] = x.items[at]
		}
		return out
	}
	slab := slabs.Get().(*util.Slab)
	defer slabs.Put(slab)
	hits := make([]hit, 0, len(picked))
	for _, at := range picked {
		if score := x.score(at, words, slab); score > 0 {
			hits = append(hits, hit{at, score})
		}
	}
	slices.SortStableFunc(hits, func(a, b hit) int { return cmp.Compare(b.score, a.score) })
	out := make([]Summary, len(hits))
	for i, h := range hits {
		out[i] = x.items[h.at]
	}
	return out
}

type hit struct {
	at    int
	score int
}

func (x Index) score(at int, words []term, slab *util.Slab) int {
	total := 0
	for _, w := range words {
		best := 0
		for i := range x.fields[at] {
			score, _ := w.match(&x.fields[at][i], false, slab)
			best = max(best, score)
		}
		if best == 0 {
			return 0
		}
		total += best
	}
	return total
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
