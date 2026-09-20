package app

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/elliot40404/creds/internal/envfile"
	"github.com/elliot40404/creds/internal/render"
	"github.com/elliot40404/creds/internal/vault"
)

func projectVault(t *testing.T) *Service {
	t.Helper()
	s, _, _, _ := initVault(t)
	if _, err := s.ImportEnv(strings.NewReader("API_KEY=base\nMODE=dev\n"), "proj/env"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Add(pgEntry()); err != nil {
		t.Fatal(err)
	}
	return s
}

func projectDir(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, envfile.ProjectFile), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return dir
}

func trustedDir(t *testing.T, s *Service, body string) string {
	t.Helper()
	dir := projectDir(t, body)
	if p, err := envfile.LoadProject(dir); err == nil {
		_ = s.TrustProject(p)
	}
	return dir
}

func trustDir(t *testing.T, s *Service, dir string) {
	t.Helper()
	p, err := envfile.LoadProject(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.TrustProject(p); err != nil {
		t.Fatal(err)
	}
}

type pipePrompter struct{ *fakePrompter }

func (pipePrompter) Interactive() bool { return false }

func TestProjectTrustPrompt(t *testing.T) {
	t.Parallel()
	s := projectVault(t)
	fp := s.Prompter.(*fakePrompter)
	dir := projectDir(t, "[map]\nH = \"db/pg:host\"\n")
	if _, err := s.ProjectEnv(dir, ""); !errors.Is(err, ErrUntrustedProject) || !strings.Contains(err.Error(), "creds trust") {
		t.Fatalf("no answer: %v", err)
	}
	fp.confirms = []bool{false}
	if _, err := s.ProjectEnv(dir, ""); !errors.Is(err, ErrAborted) {
		t.Fatalf("declined: %v", err)
	}
	fp.confirms = []bool{true}
	if vars, err := s.ProjectEnv(dir, ""); err != nil || len(vars) != 1 {
		t.Fatalf("accepted: %+v %v", vars, err)
	}
	if got := fp.shown[len(fp.shown)-1]; got != "H = db/pg:host" {
		t.Fatalf("shown %q", got)
	}
	if _, err := s.ProjectEnv(dir, ""); err != nil || len(fp.confirms) != 0 {
		t.Fatalf("trusted again: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, envfile.ProjectFile), []byte("[map]\nH = \"db/pg:password\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	s.Prompter = pipePrompter{fp}
	shown := len(fp.shown)
	if _, err := s.ProjectEnv(dir, ""); !errors.Is(err, ErrUntrustedProject) || len(fp.shown) != shown {
		t.Fatalf("changed file without tty: %v", err)
	}
	trustDir(t, s, dir)
	if vars, err := s.ProjectEnv(dir, ""); err != nil || vars[0].Value != entrySecret {
		t.Fatalf("after trust: %+v %v", vars, err)
	}
}

func TestProjectEnv(t *testing.T) {
	t.Parallel()
	s := projectVault(t)
	dir := trustedDir(t, s, `env = "proj/env"
[map]
MODE = "db/pg:database"
DB_HOST = "db/pg:host"
DATABASE_URL = "db/pg|url"
`)
	vars, err := s.ProjectEnv(dir, "")
	if err != nil {
		t.Fatal(err)
	}
	want := []envfile.Var{
		{Key: "API_KEY", Value: "base"},
		{Key: "MODE", Value: "shop"},
		{Key: "DATABASE_URL", Value: "postgresql://app:" + entrySecret + "@db.internal:5433/shop?sslmode=require"},
		{Key: "DB_HOST", Value: "db.internal"},
	}
	if len(vars) != len(want) {
		t.Fatalf("vars %+v", vars)
	}
	for i := range want {
		if vars[i] != want[i] {
			t.Fatalf("var %d: %+v", i, vars[i])
		}
	}
}

func TestProjectEnvArgWins(t *testing.T) {
	t.Parallel()
	s := projectVault(t)
	dir := projectDir(t, "env = \"missing\"\n[map]\nX = \"missing:y\"\n")
	vars, err := s.ProjectEnv(dir, "proj/env")
	if err != nil || len(vars) != 2 {
		t.Fatalf("vars %+v %v", vars, err)
	}
	vars, err = s.ProjectEnv(t.TempDir(), "proj/env")
	if err != nil || len(vars) != 2 {
		t.Fatalf("no project: %+v %v", vars, err)
	}
}

func TestProjectEnvErrors(t *testing.T) {
	t.Parallel()
	s := projectVault(t)
	s.Config.Render.Formats = map[string]string{"postgres.jdbc": "jdbc:{{.host}}"}
	if _, err := s.ProjectEnv(t.TempDir(), ""); !errors.Is(err, ErrNoEnvPath) {
		t.Fatalf("no project: %v", err)
	}
	cases := []struct {
		body string
		err  error
		msg  string
	}{
		{"[map]\nX = \"missing:y\"", vault.ErrNotFound, "map.X"},
		{"[map]\nX = \"db/pg:nope\"", ErrNoField, "map.X"},
		{"[map]\nX = \"db/pg|nope\"", render.ErrUnknownFormat, "map.X"},
		{"[map]\nX = \"proj/env|url\"", ErrNotDatabase, "map.X"},
		{"env = \"db/pg\"", ErrNotEnv, "env"},
		{"env = \"gone\"", vault.ErrNotFound, "env"},
		{"[map]\nX = \"nocolon\"", envfile.ErrBadRef, ""},
	}
	for _, c := range cases {
		_, err := s.ProjectEnv(trustedDir(t, s, c.body), "")
		if !errors.Is(err, c.err) || !strings.Contains(err.Error(), c.msg) {
			t.Fatalf("%q: %v", c.body, err)
		}
		noSecret(t, err, entrySecret)
	}
	vars, err := s.ProjectEnv(trustedDir(t, s, "[map]\nJ = \"db/pg|jdbc\""), "")
	if err != nil || len(vars) != 1 || vars[0].Value != "jdbc:db.internal" {
		t.Fatalf("override %+v %v", vars, err)
	}
}

func TestProjectTrustPromptFlagsRiskyKeys(t *testing.T) {
	t.Parallel()
	s := projectVault(t)
	fp := s.Prompter.(*fakePrompter)
	dir := projectDir(t, "[map]\nHTTPS_PROXY = \"db/pg:host\"\nHOST = \"db/pg:host\"\n")
	fp.confirms = []bool{false}
	if _, err := s.ProjectEnv(dir, ""); !errors.Is(err, ErrAborted) {
		t.Fatalf("declined: %v", err)
	}
	shown := fp.shown[len(fp.shown)-1]
	for line := range strings.Lines(shown) {
		risky := strings.Contains(line, "risky")
		if strings.HasPrefix(strings.TrimSpace(line), "HTTPS_PROXY") != risky {
			t.Fatalf("shown %q", shown)
		}
	}
}
