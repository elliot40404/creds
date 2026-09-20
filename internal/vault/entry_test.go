package vault

import (
	"encoding/json/v2"
	"errors"
	"testing"
	"time"
	"uuid"
)

func sampleEntry() Entry {
	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	return Entry{
		ID:       uuid.NewV7(),
		Path:     "work/db",
		Type:     TypeDatabase,
		Username: "admin",
		Tags:     []string{"prod"},
		Fields:   []Field{{Name: "password", Value: "hunter2", Secret: true}},
		Params:   map[string]string{"port": "5432"},
		Created:  now,
		Updated:  now,
	}
}

func TestEntryJSONRoundtrip(t *testing.T) {
	e := sampleEntry()
	data, err := json.Marshal(e)
	if err != nil {
		t.Fatal(err)
	}
	var got Entry
	if err := json.Unmarshal(data, &got, json.RejectUnknownMembers(true)); err != nil {
		t.Fatal(err)
	}
	if got.ID != e.ID || got.Path != e.Path || got.Fields[0] != e.Fields[0] || got.Params["port"] != "5432" || !got.Created.Equal(e.Created) {
		t.Fatalf("got %+v", got)
	}
}

func TestEntryValidate(t *testing.T) {
	if err := sampleEntry().Validate(); err != nil {
		t.Fatal(err)
	}
	bad := map[string]func(*Entry){
		"empty path":  func(e *Entry) { e.Path = "" },
		"bad type":    func(e *Entry) { e.Type = "nope" },
		"empty field": func(e *Entry) { e.Fields = []Field{{Value: "x"}} },
	}
	for name, mutate := range bad {
		e := sampleEntry()
		mutate(&e)
		if err := e.Validate(); !errors.Is(err, ErrInvalidEntry) {
			t.Fatalf("%s: got %v", name, err)
		}
	}
}

func TestEntryCloneIsDeep(t *testing.T) {
	e := sampleEntry()
	c := e.Clone()
	c.Tags[0] = "x"
	c.Fields[0].Value = "x"
	c.Params["port"] = "x"
	if e.Tags[0] != "prod" || e.Fields[0].Value != "hunter2" || e.Params["port"] != "5432" {
		t.Fatal("clone shares memory")
	}
}

func TestValidatePath(t *testing.T) {
	for _, p := range []string{"web/github", "a", "a.b/c..d", "team x/db", ".env/x", "..a", "a/-b", "a-"} {
		if err := ValidatePath(p); err != nil {
			t.Errorf("%q: %v", p, err)
		}
	}
	for _, p := range []string{"--show", "-x/y", "../etc/passwd", "/etc/passwd", "a/./b", "a/..", "a//b", "a/", " a", "a /b", "a/ b", "."} {
		if err := ValidatePath(p); !errors.Is(err, ErrBadPath) || !errors.Is(err, ErrInvalidEntry) {
			t.Errorf("%q: %v", p, err)
		}
	}
}

func TestEntryFieldPrefersFieldsOverParams(t *testing.T) {
	e := Entry{
		Fields: []Field{{Name: "engine", Value: "mongo"}},
		Params: map[string]string{"engine": "postgres", "sslmode": "require"},
	}
	if v, ok := e.Field("engine"); !ok || v != "mongo" {
		t.Fatalf("got %q %v", v, ok)
	}
	if v, ok := e.Field("sslmode"); !ok || v != "require" {
		t.Fatalf("got %q %v", v, ok)
	}
	if v, ok := e.Field("nope"); ok || v != "" {
		t.Fatalf("got %q %v", v, ok)
	}
}

func TestValidateChecksThePath(t *testing.T) {
	for _, p := range []string{"-rf", "a//b", "a/../b", "/abs", "a/ b"} {
		e := Entry{Path: p, Type: TypeNote}
		if err := e.Validate(); !errors.Is(err, ErrBadPath) {
			t.Errorf("%q: err = %v", p, err)
		}
	}
}
