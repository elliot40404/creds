package vault

import (
	"slices"
	"testing"
	"time"
	"uuid"
)

func mergeEntry(path, pw string) Entry {
	e := sampleEntry()
	e.Path = path
	e.Fields = []Field{{Name: "password", Value: pw, Secret: true}}
	return e
}

func edited(e Entry, pw string) Entry {
	e = e.Clone()
	e.Fields[0].Value = pw
	e.Updated = e.Updated.Add(time.Minute)
	return e
}

func entryPaths(entries []Entry) []string {
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		out = append(out, e.Path)
	}
	return out
}

func mergeBuckets(base, ours, theirs []Entry) ([]Entry, []Conflict) {
	merged, conflicts := mergeByID(base, ours, theirs)
	return merged, append(conflicts, pathConflicts(merged, ours, theirs)...)
}

func mustClean(t *testing.T, base, ours, theirs []Entry) []Entry {
	t.Helper()
	merged, conflicts := mergeBuckets(base, ours, theirs)
	if len(conflicts) != 0 {
		t.Fatalf("conflicts %+v", conflicts)
	}
	return merged
}

func TestMergeOneSideChanged(t *testing.T) {
	a, b := mergeEntry("a", "1"), mergeEntry("b", "1")
	base := []Entry{a, b}
	merged := mustClean(t, base, []Entry{edited(a, "2"), b}, base)
	if merged[indexOf(merged, a.ID)].Fields[0].Value != "2" {
		t.Fatalf("ours edit lost %+v", merged)
	}
	merged = mustClean(t, base, base, []Entry{a, edited(b, "3")})
	if merged[indexOf(merged, b.ID)].Fields[0].Value != "3" {
		t.Fatalf("theirs edit lost %+v", merged)
	}
}

func TestMergeBothSidesDifferentEntries(t *testing.T) {
	a, b := mergeEntry("a", "1"), mergeEntry("b", "1")
	c, d := mergeEntry("c", "1"), mergeEntry("d", "1")
	base := []Entry{a, b}
	merged := mustClean(t, base, []Entry{edited(a, "2"), b, c}, []Entry{a, d})
	if len(merged) != 3 || indexOf(merged, b.ID) >= 0 {
		t.Fatalf("got %v", entryPaths(merged))
	}
	if merged[indexOf(merged, a.ID)].Fields[0].Value != "2" || indexOf(merged, c.ID) < 0 || indexOf(merged, d.ID) < 0 {
		t.Fatalf("got %+v", merged)
	}
}

func TestMergeDeleteOneSide(t *testing.T) {
	a, b := mergeEntry("a", "1"), mergeEntry("b", "1")
	base := []Entry{a, b}
	merged := mustClean(t, base, []Entry{b}, base)
	if len(merged) != 1 || merged[0].ID != b.ID {
		t.Fatalf("got %v", entryPaths(merged))
	}
	merged = mustClean(t, base, []Entry{b}, []Entry{b})
	if len(merged) != 1 {
		t.Fatalf("got %v", entryPaths(merged))
	}
}

func TestMergeSameChangeBothSides(t *testing.T) {
	a := mergeEntry("a", "1")
	same := edited(a, "2")
	merged := mustClean(t, []Entry{a}, []Entry{same}, []Entry{same.Clone()})
	if len(merged) != 1 || merged[0].Fields[0].Value != "2" {
		t.Fatalf("got %+v", merged)
	}
}

func TestMergeAddWithoutBase(t *testing.T) {
	a := mergeEntry("a", "1")
	merged := mustClean(t, nil, nil, []Entry{a})
	if len(merged) != 1 || merged[0].ID != a.ID {
		t.Fatalf("got %+v", merged)
	}
}

func TestMergeEditConflict(t *testing.T) {
	a := mergeEntry("a", "1")
	ours, theirs := edited(a, "2"), edited(a, "3")
	theirs.Path = "renamed"
	merged, conflicts := mergeBuckets([]Entry{a}, []Entry{ours}, []Entry{theirs})
	if len(conflicts) != 1 {
		t.Fatalf("conflicts %+v", conflicts)
	}
	c := conflicts[0]
	if c.ID != a.ID || c.Ours.Fields[0].Value != "2" || c.Theirs.Fields[0].Value != "3" {
		t.Fatalf("conflict %+v", c)
	}
	if len(c.Paths) != 2 || c.Paths[0] != "a" || c.Paths[1] != "renamed" {
		t.Fatalf("paths %v", c.Paths)
	}
	if len(merged) != 1 || merged[0].Fields[0].Value != "2" {
		t.Fatalf("merged %+v", merged)
	}
}

func TestMergeDeleteVsEditConflict(t *testing.T) {
	a := mergeEntry("a", "1")
	_, conflicts := mergeBuckets([]Entry{a}, nil, []Entry{edited(a, "2")})
	if len(conflicts) != 1 || conflicts[0].Ours != nil || conflicts[0].Theirs == nil || conflicts[0].ID != a.ID {
		t.Fatalf("conflicts %+v", conflicts)
	}
	_, conflicts = mergeBuckets([]Entry{a}, []Entry{edited(a, "2")}, nil)
	if len(conflicts) != 1 || conflicts[0].Ours == nil || conflicts[0].Theirs != nil {
		t.Fatalf("conflicts %+v", conflicts)
	}
}

func TestMergeAddAddDifferentConflict(t *testing.T) {
	a := mergeEntry("a", "1")
	_, conflicts := mergeBuckets(nil, []Entry{a}, []Entry{edited(a, "2")})
	if len(conflicts) != 1 || conflicts[0].ID != a.ID {
		t.Fatalf("conflicts %+v", conflicts)
	}
}

func TestMergePathClash(t *testing.T) {
	a, b := mergeEntry("same", "1"), mergeEntry("same", "2")
	merged, conflicts := mergeBuckets(nil, []Entry{a}, []Entry{b})
	if len(merged) != 2 || len(conflicts) != 1 {
		t.Fatalf("merged %v conflicts %+v", entryPaths(merged), conflicts)
	}
	c := conflicts[0]
	if c.ID != uuid.Nil() || c.Paths[0] != "same" || c.Ours.ID != a.ID || c.Theirs.ID != b.ID {
		t.Fatalf("conflict %+v", c)
	}
}

func TestMergeDoesNotAlias(t *testing.T) {
	a := mergeEntry("a", "1")
	ours := []Entry{a}
	merged := mustClean(t, nil, ours, nil)
	merged[0].Fields[0].Value = "changed"
	if ours[0].Fields[0].Value != "1" {
		t.Fatal("merge output aliases input")
	}
}

func indexOf(entries []Entry, id uuid.UUID) int {
	return slices.IndexFunc(entries, func(e Entry) bool { return e.ID == id })
}
