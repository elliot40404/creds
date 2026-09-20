package app

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/elliot40404/creds/internal/config"
	"github.com/elliot40404/creds/internal/render"
)

func TestSetupFreshAndCreateVault(t *testing.T) {
	t.Parallel()
	s, fp, _ := newService(t)
	u := s.Setup()
	if err := u.Service.CheckFresh(); err != nil {
		t.Fatalf("fresh: %v", err)
	}
	code := ""
	fp.passwords = []string{mainWord, mainWord}
	s.Prompter = &codeEcho{fakePrompter: fp, code: &code}
	if err := u.Service.Init(); err != nil {
		t.Fatalf("create: %v", err)
	}
	if len(fp.shown) != 1 || fp.shown[0] != code || code == "" {
		t.Fatal("recovery code not shown once")
	}
	if err := u.Service.CheckFresh(); !errors.Is(err, ErrVaultExists) {
		t.Fatalf("want ErrVaultExists, got %v", err)
	}
}

func TestSetupSaveSettings(t *testing.T) {
	t.Parallel()
	s, _, _ := newService(t)
	u := s.Setup()
	want := Settings{Idle: time.Minute, Hard: time.Hour, Shell: render.Bash}
	if err := u.SaveSettings(want); err != nil {
		t.Fatalf("save: %v", err)
	}
	if got := u.Settings(); got != want {
		t.Fatalf("settings = %+v, want %+v", got, want)
	}
	c, err := config.Load(s.Paths.Config())
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if c.Session.Idle != want.Idle || c.Session.Hard != want.Hard || c.Render.Shell != want.Shell {
		t.Fatalf("loaded = %+v", c)
	}
}

func TestSetupSaveSettingsInvalid(t *testing.T) {
	t.Parallel()
	s, _, _ := newService(t)
	if err := s.Setup().SaveSettings(Settings{Idle: time.Hour, Hard: time.Minute, Shell: render.Bash}); err == nil {
		t.Fatal("want validation error")
	}
}

func TestSetupNextSteps(t *testing.T) {
	t.Parallel()
	s, _, _ := newService(t)
	s.Config.Render.Shell = ""
	steps := s.Setup().NextSteps()
	if len(steps) == 0 {
		t.Fatal("no next steps")
	}
	joined := strings.Join(steps, " ")
	for _, want := range []string{"creds help agents", completionStep(render.DefaultShell())} {
		if !strings.Contains(joined, want) {
			t.Fatalf("steps %q missing %q", joined, want)
		}
	}
}

func TestShellChoicesPutsCurrentFirst(t *testing.T) {
	t.Parallel()
	for _, cur := range []render.Shell{render.Bash, render.Pwsh} {
		if got := ShellChoices(cur); got[0] != string(cur) || len(got) != 2 {
			t.Fatalf("choices for %s = %v", cur, got)
		}
	}
	if got := ShellChoices("zsh"); got[0] != string(render.Bash) {
		t.Fatalf("choices = %v", got)
	}
}
