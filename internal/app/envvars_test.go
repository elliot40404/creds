package app

import (
	"errors"
	"strings"
	"testing"

	"github.com/elliot40404/creds/internal/envfile"
	"github.com/elliot40404/creds/internal/vault"
)

const envSrc = "export API_KEY=" + entrySecret + "\nMODE=dev\nMODE=prod\n"

func TestImportEnv(t *testing.T) {
	t.Parallel()
	s, _, _, _ := initVault(t)
	warns, err := s.ImportEnv(strings.NewReader(envSrc), "proj/env")
	if err != nil || len(warns) != 1 || !strings.Contains(warns[0], "MODE") {
		t.Fatalf("warns %v %v", warns, err)
	}
	e, err := s.Get("proj/env")
	if err != nil || e.Type != vault.TypeEnv {
		t.Fatalf("get %+v %v", e, err)
	}
	want := []vault.Field{
		{Name: "API_KEY", Value: entrySecret, Secret: true},
		{Name: "MODE", Value: "prod", Secret: true},
	}
	if len(e.Fields) != len(want) || e.Fields[0] != want[0] || e.Fields[1] != want[1] {
		t.Fatalf("fields %+v", e.Fields)
	}
	vars, err := s.envVars("proj/env")
	if err != nil || len(vars) != 2 || vars[0] != (envfile.Var{Key: "API_KEY", Value: entrySecret}) || vars[1].Value != "prod" {
		t.Fatalf("vars %+v %v", vars, err)
	}
	if _, err := s.ImportEnv(strings.NewReader(envSrc), "proj/env"); !errors.Is(err, vault.ErrDuplicatePath) {
		t.Fatalf("dup: %v", err)
	}
	if _, err := s.ImportEnv(strings.NewReader("1BAD=x"), "proj/bad"); !errors.Is(err, envfile.ErrBadKey) {
		t.Fatalf("bad: %v", err)
	}
	if _, err := s.Get("proj/bad"); !errors.Is(err, vault.ErrNotFound) {
		t.Fatalf("bad entry stored: %v", err)
	}
}

func TestEnvVarsRejects(t *testing.T) {
	t.Parallel()
	s, _, _, _ := initVault(t)
	if _, err := s.Add(dbEntry("db")); err != nil {
		t.Fatal(err)
	}
	if _, err := s.envVars("db"); !errors.Is(err, ErrNotEnv) {
		t.Fatalf("not env: %v", err)
	}
	if _, err := s.envVars("missing"); !errors.Is(err, vault.ErrNotFound) {
		t.Fatalf("missing: %v", err)
	}
}
