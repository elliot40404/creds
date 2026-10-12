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
	Frecency float64
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
	fields [][]field
}

type field struct {
	text   util.Chars
	weight int
}

func NewIndex(items []Summary) Index {
	sorted := sortedByPath(items)
	size, count := 0, 0
	for _, s := range sorted {
		for f := range texts(s) {
			size += len(f)
			count++
		}
	}
	buf := make([]byte, 0, size)
	all := make([]field, 0, count)
	fields := make([][]field, len(sorted))
	for i, s := range sorted {
		from := len(all)
		for f, weight := range texts(s) {
			at := len(buf)
			buf = append(buf, f...)
			all = append(all, field{util.ToChars(buf[at:len(buf):len(buf)]), weight})
		}
		fields[i] = all[from:len(all):len(all)]
	}
	return Index{items: sorted, fields: fields}
}

func sortedByPath(items []Summary) []Summary {
	order := make([]int32, len(items))
	for i := range order {
		order[i] = int32(i)
	}
	slices.SortFunc(order, func(a, b int32) int { return byPath(items[a], items[b]) })
	out := make([]Summary, len(items))
	for i, at := range order {
		out[i] = items[at]
	}
	return out
}

func texts(s Summary) iter.Seq2[string, int] {
	return func(yield func(string, int) bool) {
		for _, f := range [...]struct {
			text   string
			weight int
		}{{s.Path, 3}, {s.Name, 3}, {string(s.Type), 1}, {s.Host, 1}, {s.Username, 1}} {
			if f.text != "" && !yield(f.text, f.weight) {
				return
			}
		}
		for _, tag := range s.Tags {
			if tag != "" && !yield(tag, 2) {
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
	out := x.search(query, order == SortFrecent)
	if order == SortUpdated || order == SortCreated {
		slices.SortStableFunc(out, byOrder(order))
	}
	return out
}

func (x Index) search(query string, frecent bool) []Summary {
	f := parseFilter(query)
	picked := x.pick(f)
	words := terms(f.Text)
	if len(words) == 0 {
		out := make([]Summary, len(picked))
		for i, at := range picked {
			out[i] = x.items[at]
		}
		if frecent {
			slices.SortStableFunc(out, func(a, b Summary) int { return cmp.Compare(b.Frecency, a.Frecency) })
		}
		return out
	}
	slab := slabs.Get().(*util.Slab)
	defer slabs.Put(slab)
	hits := x.hits(picked, words, false, frecent, slab)
	if len(hits) == 0 {
		hits = x.hits(picked, words, true, frecent, slab)
	}
	out := make([]Summary, len(hits))
	for i, h := range hits {
		out[i] = x.items[h.at]
	}
	return out
}

func (x Index) hits(picked []int, words []term, typos, frecent bool, slab *util.Slab) []hit {
	hits := make([]hit, 0, len(picked))
	for _, at := range picked {
		h := x.score(at, words, typos, slab)
		if h.score == 0 {
			continue
		}
		h.rank = float64(h.score)
		if frecent {
			h.rank *= 1 + boost(x.items[at].Frecency)
		}
		hits = append(hits, h)
	}
	slices.SortStableFunc(hits, func(a, b hit) int {
		return cmp.Or(cmp.Compare(a.typos, b.typos), cmp.Compare(b.rank, a.rank))
	})
	return hits
}

func boost(frecency float64) float64 {
	if frecency <= 0 {
		return 0
	}
	return maxBoost * frecency / (frecency + 2)
}

const maxBoost = 0.5

type hit struct {
	at    int
	score int
	typos int
	rank  float64
}

func (x Index) score(at int, words []term, typos bool, slab *util.Slab) hit {
	h := hit{at: at, score: 1}
	for i := range words {
		w := &words[i]
		best := x.best(at, w, slab)
		if best == 0 && typos && x.typo(at, w) {
			best = 1
			h.typos++
		}
		if w.not != (best == 0) {
			return hit{}
		}
		if !w.not {
			h.score += best
		}
	}
	return h
}

func (x Index) typo(at int, w *term) bool {
	for i := range x.fields[at] {
		if _, _, ok := w.typo(&x.fields[at][i].text); ok {
			return true
		}
	}
	return false
}

func (x Index) best(at int, w *term, slab *util.Slab) int {
	best := 0
	for i := range x.fields[at] {
		f := &x.fields[at][i]
		score, _ := w.match(&f.text, false, slab)
		best = max(best, score*f.weight)
	}
	return best
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
