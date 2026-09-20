package app

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/elliot40404/creds/internal/vault"
)

func stripMeta(entries []vault.Entry) []vault.Entry {
	for i := range entries {
		entries[i].ID, entries[i].Created, entries[i].Updated = uuid.Nil(), time.Time{}, time.Time{}
	}
	return entries
}

func TestImportRoundtrip(t *testing.T) {
	t.Parallel()
	src, fp := exportVault(t)
	out := filepath.Join(t.TempDir(), "x.json")
	fp.inputs = []string{ExportPhrase}
	if _, err := src.ExportPlain(out, FormatJSON, false); err != nil {
		t.Fatalf("export: %v", err)
	}
	info, err := os.Stat(out)
	if err != nil || info.Size() == 0 {
		t.Fatalf("stat %v", err)
	}
	dst, _, _, _ := initVault(t)
	f, err := os.Open(filepath.Clean(out))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	res, err := dst.Import(f, false)
	if err != nil || len(res.Added) != 3 || len(res.Skipped)+len(res.Replaced) != 0 {
		t.Fatalf("import %+v %v", res, err)
	}
	want, got := allEntries(t, src), allEntries(t, dst)
	for i := range got {
		if got[i].ID == want[i].ID {
			t.Fatal("import kept source id")
		}
	}
	if !reflect.DeepEqual(stripMeta(got), stripMeta(want)) {
		t.Fatalf("got %+v\nwant %+v", got, want)
	}
}

func TestImportCollision(t *testing.T) {
	t.Parallel()
	s, _, _, _ := initVault(t)
	old, err := s.Add(dbEntry("shop/db"))
	if err != nil {
		t.Fatal(err)
	}
	doc := `{"format":"creds-export","version":1,"entries":[` +
		`{"id":"00000000-0000-0000-0000-000000000000","path":"shop/db","type":"note","notes":"new","created":"2020-01-01T00:00:00Z","updated":"2020-01-01T00:00:00Z"},` +
		`{"id":"00000000-0000-0000-0000-000000000000","path":"shop/new","type":"note","created":"2020-01-01T00:00:00Z","updated":"2020-01-01T00:00:00Z"}]}`
	res, err := s.Import(strings.NewReader(doc), false)
	if err != nil || len(res.Skipped) != 1 || res.Skipped[0] != "shop/db" || len(res.Added) != 1 {
		t.Fatalf("skip %+v %v", res, err)
	}
	if e, _ := s.Get("shop/db"); e.Type != vault.TypeDatabase {
		t.Fatalf("overwritten without flag: %+v", e)
	}
	res, err = s.Import(strings.NewReader(doc), true)
	if err != nil || len(res.Replaced) != 2 || len(res.Added) != 0 {
		t.Fatalf("overwrite %+v %v", res, err)
	}
	e, err := s.Get("shop/db")
	if err != nil || e.Notes != "new" || e.ID != old.ID || !e.Created.Equal(old.Created) {
		t.Fatalf("replaced %+v %v", e, err)
	}
}

func TestImportRejects(t *testing.T) {
	t.Parallel()
	s, _, _, _ := initVault(t)
	for name, doc := range map[string]string{
		"unknown member": `{"format":"creds-export","version":1,"entries":[],"extra":1}`,
		"format":         `{"format":"other","version":1,"entries":[]}`,
		"version":        `{"format":"creds-export","version":2,"entries":[]}`,
		"syntax":         `{"format":`,
	} {
		if _, err := s.Import(strings.NewReader(doc), false); !errors.Is(err, ErrImportFile) {
			t.Fatalf("%s: err %v", name, err)
		}
	}
	bad := `{"format":"creds-export","version":1,"entries":[` +
		`{"id":"00000000-0000-0000-0000-000000000000","path":"ok","type":"note","created":"2020-01-01T00:00:00Z","updated":"2020-01-01T00:00:00Z"},` +
		`{"id":"00000000-0000-0000-0000-000000000000","path":"bad","type":"nope","created":"2020-01-01T00:00:00Z","updated":"2020-01-01T00:00:00Z"}]}`
	if _, err := s.Import(strings.NewReader(bad), false); !errors.Is(err, vault.ErrInvalidEntry) {
		t.Fatalf("invalid entry err %v", err)
	}
	if _, err := s.Get("ok"); !errors.Is(err, vault.ErrNotFound) {
		t.Fatalf("partial import saved: %v", err)
	}
}
