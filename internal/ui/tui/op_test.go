package tui

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestSyncKeyDuringBackgroundSyncKeepsActionsWorking(t *testing.T) {
	f := newFake()
	m := startSync(t, f)
	next, auto, _ := m.handle(autoSyncMsg{})
	m = next.(Model)
	next, _, _ = m.handle(syncMsg{})
	m = run(next.(Model), auto)
	next, cmd, _ := m.handle(openMsg{path: "web/mail"})
	if cmd == nil || strings.Contains(view(next.(Model)), "still working") {
		t.Fatalf("next action blocked: %q", view(next.(Model)))
	}
}

func TestPromptEscWhileCheckingKeepsTheList(t *testing.T) {
	f := newFake()
	m, _ := relockModel(t, f)
	m = press(m, enter)
	m = press(m, text(secretA)...)
	next, cmd := m.Update(enter)
	m = next.(Model)
	next, cmd = m.Update(cmd())
	m = next.(Model)
	pending := cmd
	m = press(m, esc)
	next, _ = m.Update(pending())
	m = next.(Model)
	if _, ok := m.top().(listScreen); !ok || m.quit {
		t.Fatalf("top %T quit %v", m.top(), m.quit)
	}
}

func TestCancelledRelockDuringAutoSyncClearsTheBadge(t *testing.T) {
	f := newFake()
	m, _ := relockModel(t, f)
	m.opts.Config = newFakeConfig()
	f.syncErr = fmt.Errorf("sync: %w", ErrNeedPassword)
	next, cmd, _ := m.handle(autoSyncMsg{})
	m = run(next.(Model), cmd)
	if !strings.Contains(view(m), relockMessage) {
		t.Fatalf("no prompt: %q", view(m))
	}
	m = press(m, esc)
	if strings.Contains(view(m), "◌ syncing") {
		t.Fatalf("badge stuck: %q", view(m))
	}
	f.syncErr = nil
	next, cmd, _ = m.handle(autoSyncMsg{})
	if _, isTick := runCmd(cmd).(autoSyncMsg); cmd == nil || isTick {
		t.Fatalf("auto sync never runs again: %q", view(next.(Model)))
	}
}

func TestCancelledRelockDuringSaveLeavesTheFormUsable(t *testing.T) {
	f := newFake()
	m := start(t, f, DefaultHooks())
	m.opts.Unlock = func(string) error { return nil }
	next, cmd := m.Update(addMsg{})
	m = run(next.(Model), cmd)
	m = press(m, enter)
	m = press(m, text("web/new")...)
	f.saveErr = fmt.Errorf("add: %w", ErrNeedPassword)
	m = press(m, ctrl('s'))
	if !strings.Contains(view(m), relockMessage) {
		t.Fatalf("no prompt: %q", view(m))
	}
	m = press(m, esc)
	if _, ok := m.top().(formScreen); !ok || strings.Contains(view(m), "saving") {
		t.Fatalf("form frozen: %q", view(m))
	}
	m = press(m, esc)
	if _, ok := m.top().(listScreen); !ok {
		t.Fatalf("esc did not leave the form: %T", m.top())
	}
}

func runCmd(cmd tea.Cmd) tea.Msg {
	if cmd == nil {
		return nil
	}
	return cmd()
}
