package tui

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
)

type watchedFake struct {
	*fakeBackend
	left time.Duration
}

func (w *watchedFake) SessionLeft() time.Duration { return w.left }

func stepNoRun(t *testing.T, m Model, msg tea.Msg) (Model, tea.Cmd) {
	t.Helper()
	next, cmd := m.Update(msg)
	got, ok := next.(Model)
	if !ok {
		t.Fatalf("model %#v", next)
	}
	return got, cmd
}

func TestExpiredSessionHidesRevealedValues(t *testing.T) {
	f := &watchedFake{fakeBackend: newFake(), left: time.Minute}
	m := start(t, f.fakeBackend, Hooks{})
	m.opts.Backend = f
	m = press(m, down, enter, down)
	m, cmd := stepNoRun(t, m, ch('r'))
	m, cmd = stepNoRun(t, m, cmd())
	m, watch := stepNoRun(t, m, cmd())
	if !strings.Contains(view(m), secretA) || watch == nil {
		t.Fatalf("reveal did not start the watch: %q", view(m))
	}
	m = m.push(newValue(showValueMsg{"web/mail.pin", secretB}))
	f.left = 0
	m, next := stepNoRun(t, m, sessionLeftMsg{})
	if v := view(m); strings.Contains(v, secretA) || strings.Contains(v, secretB) {
		t.Fatalf("secret still on screen: %q", v)
	}
	if _, ok := m.top().(detailScreen); !ok {
		t.Fatalf("top %T", m.top())
	}
	if next != nil {
		t.Fatal("watch kept running with nothing revealed")
	}
}
