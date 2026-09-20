package app

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

var errNoTool = errors.New("not found")

func lookOnly(found ...string) func(string) (string, error) {
	return func(file string) (string, error) {
		if slices.Contains(found, file) {
			return filepath.Join("/usr/bin", file), nil
		}
		return "", errNoTool
	}
}

func byName(checks []Check, name string) Check {
	for _, c := range checks {
		if c.Name == name {
			return c
		}
	}
	return Check{Name: "missing " + name}
}

func TestPreflightAllGood(t *testing.T) {
	t.Parallel()
	s, _, _ := newService(t)
	u := s.Setup()
	u.Look, u.OS = lookOnly("git", "wl-copy"), "linux"
	for _, c := range u.Preflight() {
		if !c.OK {
			t.Fatalf("%s not ok: %s", c.Name, c.Message)
		}
	}
}

func TestPreflightMissingTools(t *testing.T) {
	t.Parallel()
	s, _, _ := newService(t)
	u := s.Setup()
	u.Look, u.OS = lookOnly(), "linux"
	checks := u.Preflight()
	for _, name := range []string{"git", "clipboard"} {
		c := byName(checks, name)
		if c.OK || c.Fix == "" {
			t.Fatalf("%s: ok=%v fix=%q", name, c.OK, c.Fix)
		}
	}
	if !byName(checks, "home").OK {
		t.Fatal("home should be writable")
	}
}

func TestPreflightClipboardSkippedOffLinux(t *testing.T) {
	t.Parallel()
	s, _, _ := newService(t)
	u := s.Setup()
	u.Look, u.OS = lookOnly(), "windows"
	if !byName(u.Preflight(), "clipboard").OK {
		t.Fatal("clipboard check should pass off linux")
	}
}

func TestPreflightHomeNotWritable(t *testing.T) {
	t.Parallel()
	s, _, _ := newService(t)
	blocker := filepath.Join(t.TempDir(), "home")
	if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	s.Paths.Home = blocker
	u := s.Setup()
	u.Look, u.OS = lookOnly("git"), "linux"
	c := byName(u.Preflight(), "home")
	if c.OK || c.Fix == "" {
		t.Fatalf("home check = %+v", c)
	}
}
