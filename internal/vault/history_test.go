package vault

import (
	"testing"
	"time"
	"uuid"

	"github.com/elliot40404/creds/internal/vaultfiles"
)

func tickingVault(t *testing.T) *Vault {
	t.Helper()
	v, _, _ := initVault(t)
	at := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	v.now = func() time.Time {
		at = at.Add(time.Minute)
		return at
	}
	v.SetHistory(vaultfiles.DefaultHistory, "laptop")
	return v
}

func putNote(t *testing.T, v *Vault, e Entry, notes string) Entry {
	t.Helper()
	e.Notes = notes
	return mustPut(t, v, e)
}

func TestPutPushesAndTruncatesHistory(t *testing.T) {
	v := tickingVault(t)
	e := mustPut(t, v, Entry{Path: "a", Type: TypeNote, Notes: "v0"})
	if len(e.History) != 0 {
		t.Fatalf("new entry has history %+v", e.History)
	}
	if e.Machine != "laptop" {
		t.Fatalf("machine %q", e.Machine)
	}
	for i, notes := range []string{"v1", "v2", "v3", "v4"} {
		e = putNote(t, v, e, notes)
		if want := min(i+1, vaultfiles.DefaultHistory); len(e.History) != want {
			t.Fatalf("%s: %d revisions, want %d", notes, len(e.History), want)
		}
	}
	want := []string{"v3", "v2", "v1"}
	for i, r := range e.History {
		if r.Notes != want[i] {
			t.Fatalf("revision %d is %q, want %q", i, r.Notes, want[i])
		}
		if r.Machine != "laptop" {
			t.Fatalf("revision %d machine %q", i, r.Machine)
		}
	}
}

func TestPutWithNoValueChangeKeepsHistory(t *testing.T) {
	v := tickingVault(t)
	e := mustPut(t, v, Entry{Path: "a", Type: TypeNote, Notes: "v0"})
	e = putNote(t, v, e, "v1")
	before := len(e.History)
	e = putNote(t, v, e, "v1")
	if len(e.History) != before {
		t.Fatalf("history grew on an unchanged value: %d then %d", before, len(e.History))
	}
}

func TestHistoryZeroClearsIt(t *testing.T) {
	v := tickingVault(t)
	e := mustPut(t, v, Entry{Path: "a", Type: TypeNote, Notes: "v0"})
	e = putNote(t, v, e, "v1")
	if len(e.History) == 0 {
		t.Fatal("no history to clear")
	}
	v.SetHistory(0, "laptop")
	e = putNote(t, v, e, "v2")
	if len(e.History) != 0 {
		t.Fatalf("history kept with the limit at 0: %+v", e.History)
	}
}

func TestPutKeepsTheUpdatedToken(t *testing.T) {
	v := tickingVault(t)
	e := mustPut(t, v, Entry{Path: "a", Type: TypeNote, Notes: "v0"})
	first := e.Updated
	e = putNote(t, v, e, "v1")
	if !e.Updated.After(first) {
		t.Fatalf("updated did not move: %v then %v", first, e.Updated)
	}
	if !e.History[0].At.Equal(first) {
		t.Fatalf("revision stamped %v, want the previous updated %v", e.History[0].At, first)
	}
}

func TestEqualEntryIgnoresProvenance(t *testing.T) {
	a := Entry{Path: "a", Type: TypeNote, History: []Revision{{Notes: "old", At: time.Unix(1, 0).UTC()}}}
	b := a.Clone()
	b.History, b.Machine = nil, "desktop"
	if !equalEntry(a, b) {
		t.Fatal("history and machine must not decide a merge conflict, unionHistory keeps both")
	}
	b = a.Clone()
	b.Notes = "changed"
	if equalEntry(a, b) {
		t.Fatal("a value change reads as equal")
	}
}

func TestEqualHistorySpotsADifference(t *testing.T) {
	a := []Revision{revAt("laptop", "x", 1)}
	if equalHistory(a, nil) || equalHistory(a, []Revision{revAt("laptop", "y", 1)}) ||
		equalHistory(a, []Revision{revAt("desktop", "x", 1)}) {
		t.Fatal("equalHistory is too loose")
	}
	if !equalHistory(a, []Revision{revAt("laptop", "x", 1)}) {
		t.Fatal("equalHistory is too strict")
	}
}

func TestMergeHistoryConvergesFromBothSides(t *testing.T) {
	at := func(m int) time.Time { return time.Date(2026, 1, 1, 0, m, 0, 0, time.UTC) }
	mine := []Revision{{Machine: "laptop", At: at(3), Notes: "c"}, {Machine: "laptop", At: at(1), Notes: "a"}}
	theirs := []Revision{{Machine: "desktop", At: at(2), Notes: "b"}, {Machine: "laptop", At: at(1), Notes: "a"}}
	one := mergeHistory(mine, theirs, 3)
	two := mergeHistory(theirs, mine, 3)
	if !equalHistory(one, two) {
		t.Fatalf("merge is not symmetric:\n%+v\n%+v", one, two)
	}
	if len(one) != 3 || one[0].Notes != "c" || one[1].Notes != "b" || one[2].Notes != "a" {
		t.Fatalf("merged %+v", one)
	}
	if got := mergeHistory(mine, theirs, 2); len(got) != 2 || got[0].Notes != "c" {
		t.Fatalf("truncated %+v", got)
	}
	if got := mergeHistory(mine, theirs, 0); got != nil {
		t.Fatalf("limit 0 gave %+v", got)
	}
}

func TestCloneDeepCopiesHistory(t *testing.T) {
	e := Entry{Path: "a", History: []Revision{{Notes: "x", Tags: []string{"t"}}}}
	c := e.Clone()
	c.History[0].Tags[0] = "changed"
	if e.History[0].Tags[0] != "t" {
		t.Fatal("clone shares the revision tags")
	}
}

func revAt(machine, notes string, m int) Revision {
	return Revision{Machine: machine, At: time.Date(2026, 1, 1, 0, m, 0, 0, time.UTC), Notes: notes}
}

func TestMergeByIDUnionsHistoryFromBothSides(t *testing.T) {
	base := Entry{
		Path: "a", Type: TypeNote, Notes: "base",
		ID: uuid.NewV7(), History: []Revision{revAt("laptop", "a", 1)},
	}

	mine := base.Clone()
	mine.History = []Revision{revAt("laptop", "c", 3), revAt("laptop", "a", 1)}
	mine.Notes = "mine"

	theirs := base.Clone()
	theirs.History = []Revision{revAt("desktop", "b", 2), revAt("laptop", "a", 1)}

	one, c1 := mergeByID([]Entry{base}, []Entry{mine}, []Entry{theirs})
	two, c2 := mergeByID([]Entry{base}, []Entry{theirs}, []Entry{mine})
	if len(c1) != 0 || len(c2) != 0 {
		t.Fatalf("conflicts %v %v", c1, c2)
	}
	if !equalHistory(one[0].History, two[0].History) {
		t.Fatalf("history differs by side:\n%+v\n%+v", one[0].History, two[0].History)
	}
	got := one[0].History
	if len(got) != 2 || got[0].Notes != "c" || got[1].Notes != "b" {
		t.Fatalf("merged history %+v", got)
	}
}
