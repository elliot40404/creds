package app

import (
	"errors"
	"slices"
	"testing"
	"time"
	"uuid"

	"github.com/elliot40404/creds/internal/crypto"
	"github.com/elliot40404/creds/internal/gitsync"
	"github.com/elliot40404/creds/internal/search"
	"github.com/elliot40404/creds/internal/vault"
)

const entrySecret = "s3cret-db-pass"

func dbEntry(path string) vault.Entry {
	return vault.Entry{
		Path:   path,
		Type:   vault.TypeDatabase,
		Host:   "db.internal",
		Fields: []vault.Field{{Name: "password", Value: entrySecret, Secret: true}},
	}
}

func summaryPaths(s []search.Summary) []string {
	out := make([]string, len(s))
	for i, x := range s {
		out[i] = x.Path
	}
	return out
}

func TestCRUD(t *testing.T) {
	t.Parallel()
	s, fp, _, _ := initVault(t)
	in := dbEntry("work/db")
	in.ID = uuid.NewV7()
	added, err := s.Add(in)
	if err != nil {
		t.Fatal(err)
	}
	if added.ID == in.ID || added.ID == uuid.Nil() {
		t.Fatal("add kept caller id")
	}
	if _, err := s.Add(dbEntry("work/db")); !errors.Is(err, vault.ErrDuplicatePath) {
		t.Fatalf("dup: %v", err)
	}

	if err := unlockWith(t, s, fp, mainWord); err != nil {
		t.Fatal(err)
	}
	got, err := s.Get("work/db")
	if err != nil || got.ID != added.ID || got.Fields[0].Value != entrySecret {
		t.Fatalf("get after reload: %v", err)
	}

	upd := dbEntry("work/prod-db")
	upd.Host = "prod.internal"
	moved, err := s.Update("work/db", upd)
	if err != nil {
		t.Fatal(err)
	}
	if moved.ID != added.ID || !moved.Created.Equal(added.Created) {
		t.Fatal("update changed identity")
	}
	if _, err := s.Get("work/db"); !errors.Is(err, vault.ErrNotFound) {
		t.Fatalf("old path: %v", err)
	}
	if got, err := s.Get("work/prod-db"); err != nil || got.Host != "prod.internal" {
		t.Fatalf("new path: %v", err)
	}

	if _, err := s.Update("missing", upd); !errors.Is(err, vault.ErrNotFound) {
		t.Fatalf("update missing: %v", err)
	}
	if err := s.Delete("work/prod-db"); err != nil {
		t.Fatal(err)
	}
	if err := s.Delete("work/prod-db"); !errors.Is(err, vault.ErrNotFound) {
		t.Fatalf("delete twice: %v", err)
	}
	if list, err := s.List(); err != nil || len(list) != 0 {
		t.Fatalf("list after delete: %v %v", list, err)
	}
}

func TestUpdateRenameConflict(t *testing.T) {
	t.Parallel()
	s, _, _, _ := initVault(t)
	for _, p := range []string{"a", "b"} {
		if _, err := s.Add(dbEntry(p)); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.Update("a", dbEntry("b")); !errors.Is(err, vault.ErrDuplicatePath) {
		t.Fatalf("err = %v", err)
	}
	if list, _ := s.List(); !slices.Equal(summaryPaths(list), []string{"a", "b"}) {
		t.Fatalf("list = %v", summaryPaths(list))
	}
}

func TestListAndSearch(t *testing.T) {
	t.Parallel()
	s, _, _, _ := initVault(t)
	for _, p := range []string{"work/prod/db", "home/wifi", "personal/github"} {
		if _, err := s.Add(dbEntry(p)); err != nil {
			t.Fatal(err)
		}
	}
	list, err := s.List()
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(summaryPaths(list), []string{"home/wifi", "personal/github", "work/prod/db"}) {
		t.Fatalf("list = %v", summaryPaths(list))
	}
	found, err := s.Search("wpd", search.SortPath)
	if err != nil || len(found) == 0 || found[0].Path != "work/prod/db" {
		t.Fatalf("search = %v %v", summaryPaths(found), err)
	}
	if found, _ := s.Search(entrySecret, search.SortPath); len(found) != 0 {
		t.Fatal("secret searchable")
	}
}

func TestEntriesNeedUnlock(t *testing.T) {
	t.Parallel()
	s, fp, _, _ := initVault(t)
	if err := s.Lock(); err != nil {
		t.Fatal(err)
	}
	fp.passwords = tries(nextWord)
	if _, err := s.List(); !errors.Is(err, crypto.ErrWrongSecret) {
		t.Fatalf("err = %v", err)
	}
}

func TestWriteTakesSyncLock(t *testing.T) {
	t.Parallel()
	s, _, c, _ := initVault(t)
	lock, err := gitsync.AcquireLock(s.Paths.SyncLock(), c.t)
	if err != nil {
		t.Fatal(err)
	}
	waits := 0
	s.Waiting = func() {
		waits++
		v, err := s.load()
		if err != nil {
			t.Error(err)
		}
		if _, err := v.Put(dbEntry("pulled")); err != nil {
			t.Error(err)
		}
		if err := v.Save(); err != nil {
			t.Error(err)
		}
		if err := lock.Release(); err != nil {
			t.Error(err)
		}
	}
	if _, err := s.Add(dbEntry("mine")); err != nil {
		t.Fatal(err)
	}
	if waits != 1 {
		t.Fatalf("waits = %d", waits)
	}
	if gitsync.Locked(s.Paths.SyncLock(), c.t) {
		t.Fatal("lock not released")
	}
	list, err := s.List()
	if err != nil {
		t.Fatal(err)
	}
	paths := summaryPaths(list)
	slices.Sort(paths)
	if !slices.Equal(paths, []string{"mine", "pulled"}) {
		t.Fatalf("paths = %v, concurrent write lost", paths)
	}
}

func TestUpdateRefusesStaleEntry(t *testing.T) {
	t.Parallel()
	s, _, _, _ := initVault(t)
	added, err := s.Add(dbEntry("web/a"))
	if err != nil {
		t.Fatal(err)
	}
	fresh := added.Clone()
	fresh.Host = "new.internal"
	if _, err := s.Update("web/a", fresh); err != nil {
		t.Fatal(err)
	}
	stale := added.Clone()
	stale.Host = "stale.internal"
	if _, err := s.Update("web/a", stale); !errors.Is(err, ErrStale) {
		t.Fatalf("stale update: %v", err)
	}
	got, err := s.Get("web/a")
	if err != nil || got.Host != "new.internal" {
		t.Fatalf("host = %q %v", got.Host, err)
	}
	blind := added.Clone()
	blind.Updated = time.Time{}
	blind.Host = "blind.internal"
	if _, err := s.Update("web/a", blind); err != nil {
		t.Fatalf("unversioned update: %v", err)
	}
}

func TestUpdateKeepsHistory(t *testing.T) {
	s, _, _, _ := initVault(t)
	e, err := s.Add(vault.Entry{Path: "a/note", Type: vault.TypeNote, Notes: "v0"})
	if err != nil {
		t.Fatal(err)
	}
	if e.Machine == "" {
		t.Fatal("no machine recorded")
	}
	for _, notes := range []string{"v1", "v2"} {
		e.Notes = notes
		if e, err = s.Update(e.Path, e); err != nil {
			t.Fatal(err)
		}
	}
	got, err := s.Get("a/note")
	if err != nil {
		t.Fatal(err)
	}
	if len(got.History) != 2 || got.History[0].Notes != "v1" || got.History[1].Notes != "v0" {
		t.Fatalf("history %+v", got.History)
	}
}

func TestHistoryOffKeepsNone(t *testing.T) {
	s, _, _, _ := initVault(t)
	s.Config.Vault.History = 0
	e, err := s.Add(vault.Entry{Path: "a/note", Type: vault.TypeNote, Notes: "v0"})
	if err != nil {
		t.Fatal(err)
	}
	e.Notes = "v1"
	if _, err = s.Update(e.Path, e); err != nil {
		t.Fatal(err)
	}
	got, err := s.Get("a/note")
	if err != nil {
		t.Fatal(err)
	}
	if len(got.History) != 0 {
		t.Fatalf("history %+v", got.History)
	}
}

func TestRenameKeepsThePreviousVersion(t *testing.T) {
	s, _, _, _ := initVault(t)
	e, err := s.Add(vault.Entry{Path: "a/note", Type: vault.TypeNote, Notes: "v0"})
	if err != nil {
		t.Fatal(err)
	}
	e.Path, e.Notes = "b/note", "v1"
	if _, err = s.Update("a/note", e); err != nil {
		t.Fatal(err)
	}
	got, err := s.Get("b/note")
	if err != nil {
		t.Fatal(err)
	}
	if len(got.History) != 1 || got.History[0].Notes != "v0" {
		t.Fatalf("history %+v", got.History)
	}
}
