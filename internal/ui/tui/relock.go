package tui

import (
	"errors"

	tea "charm.land/bubbletea/v2"
)

var ErrNeedPassword = errors.New("vault is locked")

const relockMessage = "Session expired. Enter your master password to continue"

type (
	lockedMsg   struct{ retry tea.Cmd }
	unlockedMsg struct{ err error }
)

type outcome interface {
	failure() error
}

func guard(fn tea.Cmd) tea.Cmd {
	return func() tea.Msg {
		msg := fn()
		if o, ok := msg.(outcome); ok && errors.Is(o.failure(), ErrNeedPassword) {
			return lockedMsg{guard(fn)}
		}
		return msg
	}
}

func (m Model) locked(retry tea.Cmd) Model {
	if m.op.prompt != 0 {
		m.op.retry = tea.Batch(m.op.retry, retry)
		return m
	}
	if m.opts.Hooks.Prompt == nil || m.opts.Unlock == nil {
		return m.finish().fail(errors.New("session expired"), "run creds unlock and open creds again")
	}
	m.op.retry = retry
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
	owner, prompt := m.op.owner, m.op.prompt
	m = m.finish().drop(prompt).fail(ErrNeedPassword, "run creds unlock, then try again")
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
	m.op.retry, m.op.prompt = nil, 0
	return m.drop(prompt).note("Unlocked"), retry
}
