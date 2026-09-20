package gitsync

import (
	"context"
	"errors"
	"testing"
	"uuid"

	"github.com/elliot40404/creds/internal/vault"
)

func conflictedPair(t *testing.T) (vaultPair, *Syncer) {
	t.Helper()
	p := newVaultPair(t)
	p.put(t, p.a, secretEntry("e", "0"))
	p.sync(t, p.a, Pushed)
	p.sync(t, p.b, FastForwarded)
	p.edit(t, p.a, setPassword("a"))
	p.sync(t, p.a, Pushed)
	p.edit(t, p.b, setPassword("b"))
	s := newSyncer(t, p, p.b)
	if _, err := s.Sync(context.Background()); !errors.Is(err, ErrConflict) {
		t.Fatalf("want conflict, got %v", err)
	}
	return p, s
}

func TestResolveThenSync(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		side vault.Side
		want string
	}{{vault.Mine, "b"}, {vault.Theirs, "a"}} {
		p, s := conflictedPair(t)
		if err := s.Resolve("e", tc.side); err != nil {
			t.Fatal(err)
		}
		st := mustState(t, s.StatePath)
		if len(st.Pending()) != 0 || st.Conflicts[0].Choice != tc.side {
			t.Fatalf("state %+v", st)
		}
		res, err := s.Sync(context.Background())
		if err != nil || res != Merged {
			t.Fatalf("sync %v %v", res, err)
		}
		if st := mustState(t, s.StatePath); st.Conflicts != nil || st.LastResult != "merged" {
			t.Fatalf("state %+v", st)
		}
		p.sync(t, p.a, FastForwarded)
		if p.password(t, p.a, "e") != tc.want || p.password(t, p.b, "e") != tc.want {
			t.Fatalf("side %d: want %s", tc.side, tc.want)
		}
	}
}

func TestResolveErrors(t *testing.T) {
	t.Parallel()
	_, s := conflictedPair(t)
	if err := s.Resolve("missing", vault.Mine); !errors.Is(err, ErrNoConflict) {
		t.Fatalf("want ErrNoConflict, got %v", err)
	}
	if err := s.Resolve("e", 0); !errors.Is(err, ErrBadSide) {
		t.Fatalf("want ErrBadSide, got %v", err)
	}
}

func TestResolveKeptWhileOtherConflictsRemain(t *testing.T) {
	t.Parallel()
	p, s := conflictedPair(t)
	commitPasswordFile(t, p, p.a)
	p.sync(t, p.a, Pushed)
	writeFile(t, p.b, "identity.pw.age", ageBlob+"other")
	sealManifest(t, p.b, p.id)
	mustCommit(t, p.b, "passwd")
	if err := s.Resolve("e", vault.Theirs); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Sync(context.Background()); !errors.Is(err, ErrConflict) {
		t.Fatalf("want conflict, got %v", err)
	}
	st := mustState(t, s.StatePath)
	pending := st.Pending()
	if len(st.Conflicts) != 2 || len(pending) != 1 || pending[0].File != "identity.pw.age" {
		t.Fatalf("state %+v", st)
	}
	if err := s.Resolve("identity.pw.age", vault.Mine); err != nil {
		t.Fatal(err)
	}
	if res, err := s.Sync(context.Background()); err != nil || res != Merged {
		t.Fatalf("sync %v %v", res, err)
	}
	if p.password(t, p.b, "e") != "a" {
		t.Fatal("entry choice lost")
	}
}

func TestMergeReportsFileAndEntryConflictsTogether(t *testing.T) {
	t.Parallel()
	p := newVaultPair(t)
	p.put(t, p.a, secretEntry("e", "0"))
	p.sync(t, p.a, Pushed)
	p.sync(t, p.b, FastForwarded)
	p.edit(t, p.a, setPassword("a"))
	commitPasswordFile(t, p, p.a)
	p.sync(t, p.a, Pushed)
	p.edit(t, p.b, setPassword("b"))
	writeFile(t, p.b, "identity.pw.age", ageBlob+"other")
	sealManifest(t, p.b, p.id)
	mustCommit(t, p.b, "passwd")
	got := p.conflicts(t, p.b, p.opts())
	if len(got) != 2 {
		t.Fatalf("conflicts %+v", got)
	}
}

func TestRecordDropsChoicesWhoseConflictIsGone(t *testing.T) {
	t.Parallel()
	id := uuid.NewV7()
	chosen := PendingConflict{Paths: []string{"e"}, IDs: []uuid.UUID{id}, Choice: vault.Mine}
	file := Conflict{File: "identity.pw.age"}
	entry := Conflict{Entry: &vault.Conflict{ID: id, Paths: []string{"e"}}}
	for _, tc := range []struct {
		live []Conflict
		want int
	}{{[]Conflict{file}, 1}, {[]Conflict{file, entry}, 2}} {
		st := State{Conflicts: []PendingConflict{chosen}}
		st.record(fixedNow, UpToDate, &ConflictError{Conflicts: []Conflict{file}, Live: tc.live})
		if len(st.Conflicts) != tc.want || st.Conflicts[len(st.Conflicts)-1].File != "identity.pw.age" {
			t.Fatalf("live %d: conflicts %+v", len(tc.live), st.Conflicts)
		}
	}
}
