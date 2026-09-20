package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/elliot40404/creds/internal/config"
)

func layoutModel(l Layout) Model {
	f := newFake()
	m := New(Options{Backend: f, Copy: f.copy, VaultPath: "/v", Layout: l})
	return step(m, tea.WindowSizeMsg{Width: 120, Height: 40})
}

func TestInlineCapsHeight(t *testing.T) {
	m := layoutModel(Layout{Inline: true, AltScreen: true, Height: 15})
	if m.height != 15 {
		t.Fatalf("height %d, want 15", m.height)
	}
	if got := strings.Count(view(m), "\n") + 1; got > 15 {
		t.Fatalf("drew %d lines, want at most 15", got)
	}
}

func TestInlineNeverBelowMinHeight(t *testing.T) {
	m := layoutModel(Layout{Inline: true, AltScreen: true, Height: 2})
	if m.height != config.MinHeight {
		t.Fatalf("height %d, want %d", m.height, config.MinHeight)
	}
}

func TestInlineKeepsShortTerminals(t *testing.T) {
	f := newFake()
	m := New(Options{Backend: f, Copy: f.copy, VaultPath: "/v", Layout: Layout{Inline: true, AltScreen: true, Height: 30}})
	m = step(m, tea.WindowSizeMsg{Width: 120, Height: 20})
	if m.height != 20 {
		t.Fatalf("height %d, want the terminal's 20", m.height)
	}
}

func TestFullscreenIgnoresHeight(t *testing.T) {
	m := layoutModel(Layout{AltScreen: true, Height: 15})
	if m.height != 40 {
		t.Fatalf("height %d, want 40", m.height)
	}
}

func TestAltScreenFollowsLayout(t *testing.T) {
	for _, alt := range []bool{true, false} {
		for _, inline := range []bool{true, false} {
			m := layoutModel(Layout{Inline: inline, AltScreen: alt, Height: 15})
			if got := m.View().AltScreen; got != alt {
				t.Fatalf("inline=%v altscreen %v, want %v", inline, got, alt)
			}
		}
	}
}

func TestZeroLayoutFallsBackToDefault(t *testing.T) {
	f := newFake()
	if got := New(Options{Backend: f}).opts.Layout; got != DefaultLayout() {
		t.Fatalf("layout %#v, want %#v", got, DefaultLayout())
	}
}
