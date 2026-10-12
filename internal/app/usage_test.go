package app

import (
	"errors"
	"io/fs"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/elliot40404/creds/internal/vault"
)

func usedRank(t *testing.T, s *Service, e vault.Entry) float64 {
	t.Helper()
	id, err := s.identity()
	if err != nil {
		t.Fatal(err)
	}
	return s.usage().Ranks(id)[e.ID]
}

func TestGetRecordsAUse(t *testing.T) {
	t.Parallel()
	s, _, _, _ := initVault(t)
	e, err := s.Add(dbEntry("work/db"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.ImportEnv(strings.NewReader(envSrc), "proj/env"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(s.Paths.Usage()); !errors.Is(err, fs.ErrNotExist) {
		t.Fatal("adding an entry counted as a use")
	}
	for range 2 {
		if _, err := s.Get("work/db"); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.envVars("work/db"); err == nil {
		t.Fatal("db entry gave env vars")
	}
	if got := usedRank(t, s, e); got != 2 {
		t.Fatalf("rank %v, want 2, a failed env call must not count", got)
	}
	if _, err := s.envVars("proj/env"); err != nil {
		t.Fatal(err)
	}
	id, err := s.identity()
	if err != nil {
		t.Fatal(err)
	}
	if ranks := s.usage().Ranks(id); len(ranks) != 2 {
		t.Fatalf("env use not recorded %v", ranks)
	}
}

func TestFrecencyOffRecordsNothing(t *testing.T) {
	t.Parallel()
	s, _, _, _ := initVault(t)
	if _, err := s.Add(dbEntry("work/db")); err != nil {
		t.Fatal(err)
	}
	if err := s.ConfigSet("search.frecency", "false"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Get("work/db"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(s.Paths.Usage()); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("usage file written with frecency off: %v", err)
	}
}

func TestUsageFileIsAHomeFile(t *testing.T) {
	t.Parallel()
	s, _, _ := newService(t)
	if !slices.Contains(s.homeFiles(), s.Paths.Usage()) {
		t.Fatal("usage.age not cleaned with the other home files")
	}
}
