package tui

import (
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/elliot40404/creds/internal/render"
)

func returnsQuickly(t *testing.T, m Model, msg tea.Msg) {
	t.Helper()
	returned := make(chan struct{})
	go func() {
		_, _ = m.Update(msg)
		close(returned)
	}()
	select {
	case <-returned:
	case <-time.After(2 * time.Second):
		t.Fatalf("%T blocked Update", msg)
	}
}

func TestSlowRemoteWorkDoesNotBlockUpdate(t *testing.T) {
	c := newFakeConfig()
	c.block = make(chan struct{})
	defer close(c.block)
	m := startCfg(t, c)
	for _, msg := range []tea.Msg{remoteSetMsg{url: "git@x:y.git"}, remoteSaveMsg{url: "git@x:y.git"}, remoteRemoveMsg{}} {
		returnsQuickly(t, m, msg)
	}
}

func TestSlowCopyDoesNotBlockUpdate(t *testing.T) {
	block := make(chan struct{})
	defer close(block)
	m := start(t, newFake(), Hooks{})
	m.opts.Copy = func(string, string, string, render.Shell) (string, error) {
		<-block
		return "", nil
	}
	returnsQuickly(t, m, copyMsg{path: "web/mail"})
}
