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

func syncing(t *testing.T, f *fakeBackend, start tea.Msg) (Model, tea.Cmd) {
	t.Helper()
	next, cmd, _ := startSync(t, f).handle(start)
	m := next.(Model)
	if !m.op.syncing || cmd == nil {
		t.Fatalf("sync not started by %T", start)
	}
	return m, cmd
}

func TestActionsWorkWhileSyncing(t *testing.T) {
	for _, kick := range []tea.Msg{syncMsg{}, autoSyncMsg{}} {
		f := newFake()
		m, sync := syncing(t, f, kick)
		next, open, _ := m.handle(openMsg{path: "web/mail"})
		m = run(next.(Model), open)
		if _, ok := m.top().(detailScreen); !ok {
			t.Fatalf("%T: open blocked: %q", kick, view(m))
		}
		m = run(m, sync)
		if _, ok := m.top().(detailScreen); !ok || m.op.syncing || f.syncs != 1 {
			t.Fatalf("%T: after sync top %T syncing %v syncs %d", kick, m.top(), m.op.syncing, f.syncs)
		}
	}
}

func TestSyncKeyWhileAnActionRuns(t *testing.T) {
	m := startSync(t, newFake())
	next, _, _ := m.handle(openMsg{path: "web/mail"})
	next, cmd, _ := next.(Model).handle(syncMsg{})
	m = next.(Model)
	if cmd == nil || !m.op.syncing || !m.op.busy() {
		t.Fatalf("sync key blocked by the action: %q", view(m))
	}
}

func TestQuitAndLockWaitForSync(t *testing.T) {
	f := newFake()
	m, _ := syncing(t, f, syncMsg{})
	for _, msg := range []tea.Msg{quitMsg{}, lockMsg{}} {
		next, cmd, _ := m.handle(msg)
		got := next.(Model)
		if cmd != nil || got.quit || f.locked != 0 || !strings.Contains(view(got), "sync is still running") {
			t.Fatalf("%T during sync: %q", msg, view(got))
		}
	}
}

func TestRemoteChangeWaitsForSync(t *testing.T) {
	m, _ := syncing(t, newFake(), syncMsg{})
	next, cmd, _ := m.handle(remoteRemoveMsg{})
	got := next.(Model)
	if cmd != nil || got.op.busy() || !strings.Contains(view(got), "sync is still running") {
		t.Fatalf("remote removed during sync: %q", view(got))
	}
}

func TestDeleteWhileSyncingWaitsForTheSync(t *testing.T) {
	f := newFake()
	m, _ := syncing(t, f, syncMsg{})
	next, cmd, _ := m.handle(confirmMsg{path: "web/mail"})
	m = next.(Model)
	if !strings.Contains(view(m), "waiting for sync to finish") {
		t.Fatalf("no wait note: %q", view(m))
	}
	next, _, _ = m.handle(cmd())
	m = next.(Model)
	if len(f.entries) != 1 || !m.op.syncing || !strings.Contains(view(m), "Deleted web/mail") {
		t.Fatalf("delete during sync: %q", view(m))
	}
}

func TestCancelledRelockDuringSyncKeepsTheRunningAction(t *testing.T) {
	f := newFake()
	m, _ := relockModel(t, f)
	m.opts.Config = newFakeConfig()
	m.op.owner = m.topID()
	f.syncErr = fmt.Errorf("sync: %w", ErrNeedPassword)
	next, cmd, _ := m.handle(autoSyncMsg{})
	m = press(run(next.(Model), cmd), esc)
	if m.op.syncing || !m.op.busy() || m.op.prompt != 0 {
		t.Fatalf("syncing %v busy %v prompt %d", m.op.syncing, m.op.busy(), m.op.prompt)
	}
}

func runCmd(cmd tea.Cmd) tea.Msg {
	if cmd == nil {
		return nil
	}
	return cmd()
}
