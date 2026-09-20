package cli

import (
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/elliot40404/creds/internal/clipboard"
)

type fakeClip struct {
	value    string
	writeErr error
	clears   int
	stdin    string
	vars     map[string]string
	jobs     []clipboard.ClearJob
	spawnErr error
	tty      string
	errTTY   bool
	slept    time.Duration
	clearErr error
}

func (f *fakeClip) Read() (string, error) { return f.value, nil }

func (f *fakeClip) Write(v string) error {
	if f.writeErr != nil {
		return f.writeErr
	}
	f.value = v
	return nil
}

func (f *fakeClip) Clear() error {
	if f.clearErr != nil {
		return f.clearErr
	}
	f.clears++
	f.value = ""
	return nil
}

func (f *fakeClip) openTTY() (*os.File, error) {
	if f.tty == "" {
		return nil, os.ErrNotExist
	}
	return os.Create(f.tty)
}

func (f *fakeClip) env() Clip {
	return Clip{
		In:      strings.NewReader(f.stdin),
		Native:  f,
		Getenv:  func(k string) string { return f.vars[k] },
		OpenTTY: f.openTTY,
		IsTTY:   func(any) bool { return f.errTTY },
		Spawn: func(j clipboard.ClearJob) error {
			f.jobs = append(f.jobs, j)
			return f.spawnErr
		},
		Sleep: func(d time.Duration) { f.slept = d },
	}
}

func TestClipClearChild(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.clip.value = loginHidden
	h.clip.stdin = clipboard.Hash(loginHidden) + "\n"
	r := h.ok(&fake{}, clipboard.ClearCommand, "--after", "20s", "--mode", "native")
	if h.clip.clears != 1 || h.clip.slept != 20*time.Second || r.out != "" || r.err != "" {
		t.Fatalf("%+v %+v", h.clip, r)
	}
}

func TestClipClearChildKeepsNewValue(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.clip.value = "copied later"
	h.clip.stdin = clipboard.Hash(loginHidden)
	h.ok(&fake{}, clipboard.ClearCommand, "--after", "1s", "--mode", "native")
	if h.clip.clears != 0 || h.clip.value != "copied later" {
		t.Fatalf("%+v", h.clip)
	}
}

func TestClipClearChildRejects(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.clip.stdin = loginHidden
	h.fail(&fake{}, clipboard.ClearCommand, "--after", "1s", "--mode", "native")
	h.fail(&fake{}, clipboard.ClearCommand, "--after", "1s", "--mode", "x11")
	if h.clip.clears != 0 {
		t.Fatal("cleared on bad input")
	}
}

func TestClipClearHidden(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	r := h.ok(&fake{}, "--help")
	if strings.Contains(r.out, clipboard.ClearCommand) {
		t.Fatal("hidden command listed")
	}
}

func TestClipClearFailureWarnsOnTheNextRun(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.clip.value = loginHidden
	h.clip.stdin = clipboard.Hash(loginHidden) + "\n"
	h.clip.clearErr = errors.New("wl-copy: no wayland server")
	h.fail(&fake{}, clipboard.ClearCommand, "--after", "1s", "--mode", "native")
	r := h.run(&fake{}, "list")
	if !strings.Contains(r.err, "clipboard was not cleared") || !strings.Contains(r.err, "no wayland server") {
		t.Fatalf("stderr %q", r.err)
	}
	if r := h.run(&fake{}, "list"); strings.Contains(r.err, "clipboard was not cleared") {
		t.Fatalf("warned twice: %q", r.err)
	}
}
