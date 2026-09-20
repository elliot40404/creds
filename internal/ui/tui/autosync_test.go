package tui

import (
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/elliot40404/creds/internal/app"
)

func startSync(t *testing.T, f *fakeBackend) Model {
	t.Helper()
	m := start(t, f, DefaultHooks())
	m.opts.Config = newFakeConfig()
	return step(m, tea.WindowSizeMsg{Width: 80, Height: 24})
}

func TestSaveArmsASyncTick(t *testing.T) {
	f := newFake()
	m := startSync(t, f)
	_, cmd, ok := m.saved(savedMsg{entry: f.entries[0]})
	if !ok || cmd == nil {
		t.Fatal("save produced no command")
	}
	if !hasTick(t, cmd) {
		t.Fatal("save did not arm a sync tick")
	}
	_, cmd, _ = m.deleted(deletedMsg{path: "db/prod"})
	if !hasTick(t, cmd) {
		t.Fatal("delete did not arm a sync tick")
	}
}

func hasTick(t *testing.T, cmd tea.Cmd) bool {
	t.Helper()
	msg := cmd()
	batch, ok := msg.(tea.BatchMsg)
	if !ok {
		_, isTick := msg.(autoSyncMsg)
		return isTick
	}
	for _, c := range batch {
		if c == nil {
			continue
		}
		if _, isTick := c().(autoSyncMsg); isTick {
			return true
		}
	}
	return false
}

func TestAutoSyncCallsSyncOnce(t *testing.T) {
	f := newFake()
	f.syncRes = "pushed 1 commit"
	f.status = app.SyncStatus{Remote: "https://example.com/v.git"}
	m := startSync(t, f)
	next, cmd, ok := m.handle(autoSyncMsg{})
	if !ok {
		t.Fatal("autoSyncMsg not handled")
	}
	m = next.(Model)
	if m.op.kind != opAutoSync {
		t.Fatal("not marked as syncing")
	}
	if !strings.Contains(view(m), "◌ syncing") {
		t.Fatalf("header does not show syncing: %q", view(m))
	}
	m = run(m, cmd)
	if f.syncs != 1 {
		t.Fatalf("Sync called %d times", f.syncs)
	}
	if m.op.syncing() {
		t.Fatal("still syncing after the result")
	}
	if strings.Contains(view(m), "◌ syncing") {
		t.Fatalf("header still syncing: %q", view(m))
	}
}

func TestAutoSyncWhileSyncingRearmsInstead(t *testing.T) {
	f := newFake()
	m := startSync(t, f)
	next, first, _ := m.handle(autoSyncMsg{})
	m = next.(Model)
	next, cmd, _ := m.handle(autoSyncMsg{})
	m = next.(Model)
	if first == nil {
		t.Fatal("first tick did not sync")
	}
	if !hasTick(t, cmd) {
		t.Fatal("a second tick while syncing did not re-arm a tick")
	}
	first()
	next, _, _ = m.handle(autoSyncedMsg{result: "pushed"})
	m = next.(Model)
	if !m.op.idle() {
		t.Fatal("op not cleared")
	}
	if f.syncs != 1 {
		t.Fatalf("Sync called %d times, want 1", f.syncs)
	}
}

func TestAutoSyncWaitsForARunningAction(t *testing.T) {
	f := newFake()
	m := startSync(t, f)
	m.op.kind = opAction
	next, cmd, ok := m.handle(autoSyncMsg{})
	if !ok || !hasTick(t, cmd) || f.syncs != 0 {
		t.Fatal("background sync did not wait for the running action")
	}
	if strings.Contains(view(next.(Model)), "still working") {
		t.Fatalf("busy message shown: %q", view(next.(Model)))
	}
}

func TestAutoSyncedRefreshesStatusAndReportsFailure(t *testing.T) {
	f := newFake()
	f.status = app.SyncStatus{Remote: "https://example.com/v.git"}
	m := startSync(t, f)
	next, cmd, _ := m.handle(autoSyncedMsg{result: "pushed"})
	m = run(next.(Model), cmd)
	if m.status.Remote == "" {
		t.Fatal("status not refreshed after a background sync")
	}
	f.syncErr = errors.New("remote hung up")
	next, _, _ = m.handle(autoSyncedMsg{errResult: errResult{f.syncErr}})
	if !strings.Contains(view(next.(Model)), "sync failed") {
		t.Fatalf("no failure line: %q", view(next.(Model)))
	}
}

func TestSyncKeyDoesNotStackOnABackgroundSync(t *testing.T) {
	f := newFake()
	m := startSync(t, f)
	next, _, _ := m.handle(autoSyncMsg{})
	m = next.(Model)
	next, cmd, _ := m.handle(syncMsg{})
	if cmd != nil {
		t.Fatal("s key started a second sync")
	}
	if !strings.Contains(view(next.(Model)), "sync already running") {
		t.Fatalf("no note: %q", view(next.(Model)))
	}
}
