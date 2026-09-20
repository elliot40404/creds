package app

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/elliot40404/creds/internal/config"
)

func plantStray(t *testing.T, s *Service) {
	t.Helper()
	if err := os.MkdirAll(s.Paths.Home, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{s.Paths.Config(), s.Paths.Trust()} {
		if err := os.WriteFile(p, []byte("[session]\nidle = \"8760h\"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

func TestPreflightReportsStrayHome(t *testing.T) {
	t.Parallel()
	s, _, _ := newService(t)
	plantStray(t, s)
	u := s.Setup()
	u.Look, u.OS = lookOnly("git", "wl-copy"), "linux"
	c := byName(u.Preflight(), "home contents")
	if c.OK || !strings.Contains(c.Message, "config.toml") || !strings.Contains(c.Message, "trust.json") {
		t.Fatalf("home contents: ok=%v msg=%q", c.OK, c.Message)
	}
	if c.Fix == "" {
		t.Fatal("no fix hint")
	}
}

func TestPreflightHomeAccessChecked(t *testing.T) {
	t.Parallel()
	s, _, _ := newService(t)
	u := s.Setup()
	u.Look, u.OS = lookOnly("git", "wl-copy"), "linux"
	if c := byName(u.Preflight(), "home access"); !c.OK {
		t.Fatalf("home access: %s", c.Message)
	}
}

func TestInitKeepsStrayOnConfirm(t *testing.T) {
	t.Parallel()
	s, fp, _ := newService(t)
	plantStray(t, s)
	fp.confirms = []bool{true}
	fp.passwords = []string{mainWord, mainWord}
	code := ""
	s.Prompter = &codeEcho{fakePrompter: fp, code: &code}
	if err := s.Init(); err != nil {
		t.Fatalf("init: %v", err)
	}
	if _, err := os.Stat(s.Paths.Config()); err != nil {
		t.Fatalf("config.toml removed: %v", err)
	}
	if !hasWarning(s, "kept") {
		t.Fatalf("no warning, got %v", s.warnings)
	}
}

func TestInitRemovesStrayOnRefusal(t *testing.T) {
	t.Parallel()
	s, fp, _ := newService(t)
	plantStray(t, s)
	s.Config.Session.Idle = 8760 * time.Hour
	fp.confirms = []bool{false}
	fp.passwords = []string{mainWord, mainWord}
	code := ""
	s.Prompter = &codeEcho{fakePrompter: fp, code: &code}
	if err := s.Init(); err != nil {
		t.Fatalf("init: %v", err)
	}
	for _, p := range []string{s.Paths.Config(), s.Paths.Trust()} {
		if _, err := os.Stat(p); err == nil {
			t.Fatalf("%s still there", p)
		}
	}
	if s.Config.Session.Idle != config.Default().Session.Idle {
		t.Fatalf("config not reset: %v", s.Config.Session.Idle)
	}
}

func TestInitAsksOnce(t *testing.T) {
	t.Parallel()
	s, fp, _ := newService(t)
	plantStray(t, s)
	fp.confirms = []bool{true}
	if err := s.CheckFresh(); err != nil {
		t.Fatal(err)
	}
	if err := s.CheckFresh(); err != nil {
		t.Fatal(err)
	}
	if n := countPrompts(fp, "Keep them?"); n != 1 {
		t.Fatalf("asked %d times", n)
	}
}

func TestStrayPromptExplainsConfigSet(t *testing.T) {
	t.Parallel()
	s, fp, _ := newService(t)
	plantStray(t, s)
	fp.confirms = []bool{true}
	if err := s.CheckFresh(); err != nil {
		t.Fatal(err)
	}
	if n := countPrompts(fp, "creds config set writes config.toml"); n != 1 {
		t.Fatalf("no config set note in %v", fp.prompts)
	}
	if note := s.configSetNote([]string{"trust.json"}); note != "" {
		t.Fatalf("note without config.toml: %q", note)
	}
}

func hasWarning(s *Service, want string) bool {
	for _, w := range s.warnings {
		if strings.Contains(w, want) {
			return true
		}
	}
	return false
}

func countPrompts(f *fakePrompter, want string) int {
	n := 0
	for _, p := range f.prompts {
		if strings.Contains(p, want) {
			n++
		}
	}
	return n
}
