package vault

import (
	"maps"
	"slices"
	"uuid"
)

type Conflict struct {
	ID     uuid.UUID
	Paths  []string
	Ours   *Entry
	Theirs *Entry
}

type side struct {
	entry Entry
	ok    bool
}

func mergeByID(base, ours, theirs []Entry) ([]Entry, []Conflict) {
	b, o, t := byID(base), byID(ours), byID(theirs)
	ids := slices.SortedFunc(maps.Keys(unionIDs(b, o, t)), uuid.UUID.Compare)
	var merged []Entry
	var conflicts []Conflict
	for _, id := range ids {
		bv, ov, tv := lookup(b, id), lookup(o, id), lookup(t, id)
		pick, ok := mergeOne(bv, ov, tv)
		if !ok {
			conflicts = append(conflicts, idConflict(id, ov, tv))
			pick = ov
		}
		pick = unionHistory(pick, ov, tv)
		if pick.ok {
			merged = append(merged, pick.entry.Clone())
		}
	}
	return merged, conflicts
}

func mergeOne(b, o, t side) (side, bool) {
	switch {
	case sameSide(o, t), sameSide(b, t):
		return o, true
	case sameSide(b, o):
		return t, true
	}
	return side{}, false
}

func idConflict(id uuid.UUID, o, t side) Conflict {
	c := Conflict{ID: id}
	if o.ok {
		c.Ours = new(o.entry.Clone())
		c.Paths = append(c.Paths, o.entry.Path)
	}
	if t.ok {
		c.Theirs = new(t.entry.Clone())
		if !slices.Contains(c.Paths, t.entry.Path) {
			c.Paths = append(c.Paths, t.entry.Path)
		}
	}
	return c
}

func pathConflicts(merged, ours, theirs []Entry) []Conflict {
	seen := map[string]int{}
	var clashes []string
	for _, e := range merged {
		seen[e.Path]++
		if seen[e.Path] == 2 {
			clashes = append(clashes, e.Path)
		}
	}
	var out []Conflict
	for _, p := range clashes {
		c := Conflict{Paths: []string{p}}
		if e, ok := byPath(ours, p); ok {
			c.Ours = new(e.Clone())
		}
		if e, ok := byPath(theirs, p); ok {
			c.Theirs = new(e.Clone())
		}
		out = append(out, c)
	}
	return out
}

func byID(entries []Entry) map[uuid.UUID]Entry {
	m := make(map[uuid.UUID]Entry, len(entries))
	for _, e := range entries {
		m[e.ID] = e
	}
	return m
}

func byPath(entries []Entry, path string) (Entry, bool) {
	i := slices.IndexFunc(entries, func(e Entry) bool { return e.Path == path })
	if i < 0 {
		return Entry{}, false
	}
	return entries[i], true
}

func unionIDs(sets ...map[uuid.UUID]Entry) map[uuid.UUID]struct{} {
	out := map[uuid.UUID]struct{}{}
	for _, s := range sets {
		for id := range s {
			out[id] = struct{}{}
		}
	}
	return out
}

func lookup(m map[uuid.UUID]Entry, id uuid.UUID) side {
	e, ok := m[id]
	return side{entry: e, ok: ok}
}

func sameSide(a, b side) bool {
	if a.ok != b.ok {
		return false
	}
	return !a.ok || equalEntry(a.entry, b.entry)
}

func equalEntry(a, b Entry) bool {
	return a.ID == b.ID && a.Path == b.Path && a.Type == b.Type &&
		a.Created.Equal(b.Created) && a.Updated.Equal(b.Updated) && sameValues(a.revision(), b.revision())
}

func equalHistory(a, b []Revision) bool {
	return slices.EqualFunc(a, b, func(x, y Revision) bool {
		return x.Machine == y.Machine && x.At.Equal(y.At) && sameValues(x, y)
	})
}

func unionHistory(pick, o, t side) side {
	if !pick.ok || !o.ok || !t.ok || equalHistory(o.entry.History, t.entry.History) {
		return pick
	}
	limit := max(len(o.entry.History), len(t.entry.History))
	pick.entry = pick.entry.Clone()
	pick.entry.History = mergeHistory(o.entry.History, t.entry.History, limit)
	return pick
}
