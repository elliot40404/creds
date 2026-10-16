package tui

import (
	"reflect"
	"sync"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
)

const loopTimeout = 20 * time.Second

type loop struct {
	t       *testing.T
	m       Model
	msgs    chan tea.Msg
	done    chan struct{}
	wg      sync.WaitGroup
	pending int
}

func newLoop(t *testing.T, m Model) *loop {
	l := &loop{t: t, m: m, msgs: make(chan tea.Msg), done: make(chan struct{})}
	t.Cleanup(func() {
		close(l.done)
		l.wg.Wait()
	})
	return l
}

func (l *loop) exec(cmds ...tea.Cmd) {
	for _, cmd := range cmds {
		if cmd == nil {
			continue
		}
		l.pending++
		l.wg.Go(func() { l.deliver(cmd()) })
	}
}

func (l *loop) sequence(cmds []tea.Cmd) {
	l.pending += len(cmds)
	l.wg.Go(func() {
		for _, cmd := range cmds {
			var msg tea.Msg
			if cmd != nil {
				msg = cmd()
			}
			l.deliver(msg)
		}
	})
}

func (l *loop) deliver(msg tea.Msg) {
	select {
	case l.msgs <- msg:
	case <-l.done:
	}
}

func (l *loop) send(msg tea.Msg) {
	switch msg := msg.(type) {
	case nil, tea.QuitMsg:
	case tea.BatchMsg:
		l.exec(msg...)
	default:
		if cmds, ok := sequenceCmds(msg); ok {
			l.sequence(cmds)
			return
		}
		next, cmd := l.m.Update(msg)
		l.m = next.(Model)
		l.exec(cmd)
	}
}

func sequenceCmds(msg tea.Msg) ([]tea.Cmd, bool) {
	v := reflect.ValueOf(msg)
	if v.Kind() != reflect.Slice || v.Type().Elem() != reflect.TypeFor[tea.Cmd]() {
		return nil, false
	}
	return v.Convert(reflect.TypeFor[[]tea.Cmd]()).Interface().([]tea.Cmd), true
}

func (l *loop) press(ks ...tea.KeyPressMsg) {
	for _, k := range ks {
		l.send(k)
	}
}

func (l *loop) next(timeout <-chan time.Time, what string) {
	l.t.Helper()
	select {
	case msg := <-l.msgs:
		l.pending--
		l.send(msg)
	case <-timeout:
		l.t.Fatalf("timed out waiting for %s: %q", what, view(l.m))
	}
}

func (l *loop) until(what string, ok func(Model) bool) {
	l.t.Helper()
	timeout := time.After(loopTimeout)
	for !ok(l.m) {
		l.next(timeout, what)
	}
}

func (l *loop) settle() {
	l.t.Helper()
	timeout := time.After(loopTimeout)
	for l.pending > 0 {
		l.next(timeout, "commands to finish")
	}
}
