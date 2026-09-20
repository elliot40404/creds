package app

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/elliot40404/creds/internal/render"
	"github.com/elliot40404/creds/internal/vault"
)

func pgEntry() vault.Entry {
	return vault.Entry{
		Path:     "db/pg",
		Type:     vault.TypeDatabase,
		Host:     "db.internal",
		Username: "app",
		Params:   map[string]string{"sslmode": "require"},
		Fields: []vault.Field{
			{Name: "engine", Value: render.Postgres},
			{Name: "port", Value: "5433"},
			{Name: "database", Value: "shop"},
			{Name: "password", Value: entrySecret, Secret: true},
		},
	}
}

func TestRender(t *testing.T) {
	t.Parallel()
	s, _, _, _ := initVault(t)
	s.Config.Render.Shell = render.Bash
	if _, err := s.Add(pgEntry()); err != nil {
		t.Fatal(err)
	}
	got, err := s.Render("db/pg", "url", "")
	want := "postgresql://app:" + entrySecret + "@db.internal:5433/shop?sslmode=require"
	if err != nil || got != want {
		t.Fatalf("url %q %v", got, err)
	}
	got, err = s.Render("db/pg", "psql", "")
	want = "PGPASSWORD=" + entrySecret + " PGSSLMODE=require psql -h db.internal -p 5433 -U app -d shop"
	if err != nil || got != want {
		t.Fatalf("psql %q %v", got, err)
	}
	if _, err := s.Render("db/pg", "nope", ""); !errors.Is(err, render.ErrUnknownFormat) {
		t.Fatalf("unknown format: %v", err)
	}
	if _, err := s.Render("missing", "url", ""); !errors.Is(err, vault.ErrNotFound) {
		t.Fatalf("missing: %v", err)
	}
}

func TestRenderOverride(t *testing.T) {
	t.Parallel()
	s, _, _, _ := initVault(t)
	s.Config.Render.Formats = map[string]string{
		"postgres.url":  "pg://{{.host}}",
		"postgres.jdbc": "jdbc:postgresql://{{.host}}:{{.port}}/{{.database}}",
	}
	if _, err := s.Add(pgEntry()); err != nil {
		t.Fatal(err)
	}
	if got, err := s.Render("db/pg", "url", ""); err != nil || got != "pg://db.internal" {
		t.Fatalf("override %q %v", got, err)
	}
	if got, err := s.Render("db/pg", "jdbc", ""); err != nil || got != "jdbc:postgresql://db.internal:5433/shop" {
		t.Fatalf("custom %q %v", got, err)
	}
	names, err := s.Formats("db/pg")
	if err != nil || !slices.Contains(names, "jdbc") || !slices.Contains(names, "psql") || !slices.IsSorted(names) {
		t.Fatalf("formats %v %v", names, err)
	}
}

func TestRenderNotDatabase(t *testing.T) {
	t.Parallel()
	s, _, _, _ := initVault(t)
	e := vault.Entry{Path: "web", Type: vault.TypeLogin}
	if _, err := s.Add(e); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Render("web", "url", ""); !errors.Is(err, ErrNotDatabase) {
		t.Fatalf("render: %v", err)
	}
	if _, err := s.Formats("web"); !errors.Is(err, ErrNotDatabase) {
		t.Fatalf("formats: %v", err)
	}
}

func TestRenderNoEngine(t *testing.T) {
	t.Parallel()
	s, _, _, _ := initVault(t)
	if _, err := s.Add(dbEntry("db/old")); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Formats("db/old"); !errors.Is(err, render.ErrUnknownEngine) {
		t.Fatalf("formats: %v", err)
	}
}

func TestRenderShellOverrideAndVaries(t *testing.T) {
	t.Parallel()
	s, _, _, _ := initVault(t)
	s.Config.Render.Shell = render.Bash
	if _, err := s.Add(pgEntry()); err != nil {
		t.Fatal(err)
	}
	if got := s.RenderShell(); got != render.Bash {
		t.Fatalf("shell %q", got)
	}
	got, err := s.Render("db/pg", "psql", render.Pwsh)
	if err != nil || !strings.HasPrefix(got, "$env:PGPASSWORD=") {
		t.Fatalf("pwsh %q %v", got, err)
	}
	for format, want := range map[string]bool{"url": false, "dotenv": false, "psql": true, "env": true} {
		varies, err := s.ShellVaries("db/pg", format)
		if err != nil || varies != want {
			t.Fatalf("%s varies %v want %v, err %v", format, varies, want, err)
		}
	}
	if _, err := s.ShellVaries("db/pg", "nope"); !errors.Is(err, render.ErrUnknownFormat) {
		t.Fatalf("unknown: %v", err)
	}
}
