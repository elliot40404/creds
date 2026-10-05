package tui

import (
	"errors"
	"slices"

	tea "charm.land/bubbletea/v2"
)

var ErrNeedPassword = errors.New("vault is locked")

const relockMessage = "Session expired. Enter your master password to continue"

type (
	lockedMsg struct {
		retry tea.Cmd
		kind  opKind
	}
	unlockedMsg struct{ err error }
)

type outcome interface {
	failure() error
}

func guard(kind opKind, fn tea.Cmd) tea.Cmd {
	return func() tea.Msg {
		msg := fn()
		if o, ok := msg.(outcome); ok && errors.Is(o.failure(), ErrNeedPassword) {
			return lockedMsg{guard(kind, fn), kind}
		}
		return msg
	}
}

func (m Model) locked(msg lockedMsg) Model {
	if m.op.prompt != 0 {
		m.op.retry = tea.Batch(m.op.retry, msg.retry)
		m.op.waiting = append(slices.Clip(m.op.waiting), msg.kind)
		return m
	}
	if m.opts.Hooks.Prompt == nil || m.opts.Unlock == nil {
		return m.release(msg.kind).fail(errors.New("session expired"), "run creds unlock and open creds again")
	}
	m.op.retry, m.op.waiting = msg.retry, []opKind{msg.kind}
	m = m.push(m.opts.Hooks.Prompt(relockMessage))
	m.op.prompt = m.topID()
	return m
}

func (m Model) unlock(res PasswordResult) (tea.Model, tea.Cmd) {
	if m.op.prompt == 0 {
		return m, nil
	}
	if res.Canceled {
		return m.cancelRelock()
	}
	unlock, pw := m.opts.Unlock, res.Password
	return m, func() tea.Msg { return unlockedMsg{unlock(pw)} }
}

func (m Model) cancelRelock() (tea.Model, tea.Cmd) {
	owner, prompt, waiting := m.op.owner, m.op.prompt, m.op.waiting
	m = m.clearRelock().drop(prompt).fail(ErrNeedPassword, "run creds unlock, then try again")
	for _, kind := range waiting {
		m = m.release(kind)
	}
	if !slices.Contains(waiting, opAction) {
		return m, nil
	}
	if i := m.index(owner); i >= 0 {
		if _, ok := m.stack[i].screen.(formScreen); ok {
			return m.sendTo(owner, FormError{Err: ErrNeedPassword})
		}
	}
	return m, nil
}

func (m Model) unlocked(msg unlockedMsg) (tea.Model, tea.Cmd) {
	prompt := m.op.prompt
	if prompt == 0 || m.index(prompt) < 0 {
		return m, nil
	}
	if msg.err != nil {
		return m.sendTo(prompt, PasswordError{Err: msg.err})
	}
	retry := m.op.retry
	return m.clearRelock().drop(prompt).note("Unlocked"), retry
}

func (m Model) clearRelock() Model {
	m.op.retry, m.op.prompt, m.op.waiting = nil, 0, nil
	return m
}
