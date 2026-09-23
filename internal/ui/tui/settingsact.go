package tui

import (
	"errors"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/elliot40404/creds/internal/app"
	"github.com/elliot40404/creds/internal/safetext"
)

type ConfigBackend interface {
	PathReport() app.PathReport
	ConfigFields() []app.ConfigField
	ConfigSet(key, value string) error
	ConfigUnset(key string) error
	ConfigPreview(key, value string) (string, error)
	RemoteURL() string
	CheckRemote(url string) error
	SetRemote(url string) error
	RemoteRemove() error
	TrustList() ([]string, error)
	Untrust(file string) error
	SyncAfter() time.Duration
	SyncDue() bool
}

type (
	settingsMsg  struct{}
	configSetMsg struct {
		key, value string
		quiet      bool
	}
	configUnsetMsg   struct{ key string }
	remoteSetMsg     struct{ url string }
	remoteSaveMsg    struct{ url string }
	remoteRemoveMsg  struct{}
	untrustMsg       struct{ file string }
	remoteCheckedMsg struct {
		errResult
		url string
	}
	appliedMsg struct {
		errResult
		done, hint string
	}
)

var errNoSettings = errors.New("settings are not available here")

func (m Model) config() (ConfigBackend, Model, bool) {
	if m.opts.Config == nil {
		return nil, m.fail(errNoSettings, "run creds config"), false
	}
	return m.opts.Config, m, true
}

func (m Model) openSettings() (tea.Model, tea.Cmd) {
	c, m, ok := m.config()
	if !ok {
		return m, nil
	}
	return m.push(newSettings(c)), nil
}

func (m Model) setConfig(msg configSetMsg) (tea.Model, tea.Cmd) {
	done := msg.key + " saved"
	if msg.quiet {
		done = ""
	}
	return m.apply(func(c ConfigBackend) error { return c.ConfigSet(msg.key, msg.value) },
		done, "check the value and try again")
}

func (m Model) unsetConfig(msg configUnsetMsg) (tea.Model, tea.Cmd) {
	return m.apply(func(c ConfigBackend) error { return c.ConfigUnset(msg.key) },
		msg.key+" removed", "only render format overrides can be removed")
}

func (m Model) checkRemote(msg remoteSetMsg) (tea.Model, tea.Cmd) {
	c, m, ok := m.config()
	if !ok {
		return m, nil
	}
	return m.note("checking the remote").start(opAction, func() tea.Msg {
		return remoteCheckedMsg{errResult{c.CheckRemote(msg.url)}, msg.url}
	})
}

func (m Model) remoteChecked(msg remoteCheckedMsg) (tea.Model, tea.Cmd, bool) {
	m = m.finish()
	if msg.err != nil {
		return m.fail(firstLine(msg.err), "pick an empty repository, or run creds remote add to attach it anyway"), nil, true
	}
	return m.note(""), send(askMsg{
		title:  "Push the whole vault to " + safetext.Remote(msg.url) + "?",
		detail: "every secret and both wrapped identity files go to that remote, so anyone who can read it can guess your master password offline",
		then:   remoteSaveMsg{url: msg.url},
	}), true
}

func (m Model) setRemote(msg remoteSaveMsg) (tea.Model, tea.Cmd) {
	return m.applyLater(func(c ConfigBackend) error { return c.SetRemote(msg.url) },
		"remote saved", "pass a git url like git@github.com:you/vault.git")
}

func (m Model) confirmed(msg confirmedMsg) (tea.Model, tea.Cmd) {
	m = m.drop(m.topID())
	if run := dispatch(msg.then); run != nil {
		return run(m)
	}
	return m, nil
}

func (m Model) removeRemote() (tea.Model, tea.Cmd) {
	return m.applyLater(ConfigBackend.RemoteRemove, "remote removed", "run creds remote remove")
}

func (m Model) untrust(msg untrustMsg) (tea.Model, tea.Cmd) {
	return m.apply(func(c ConfigBackend) error { return c.Untrust(msg.file) },
		"forgot "+msg.file, "run creds trust list")
}

func (m Model) apply(fn func(ConfigBackend) error, done, hint string) (tea.Model, tea.Cmd) {
	c, m, ok := m.config()
	if !ok {
		return m, nil
	}
	if err := fn(c); err != nil {
		return m.refresh(c).fail(firstLine(err), hint), nil
	}
	if m = m.refresh(c); done != "" {
		m = m.note("%s", done)
	}
	return m, nil
}

func (m Model) applyLater(fn func(ConfigBackend) error, done, hint string) (tea.Model, tea.Cmd) {
	c, m, ok := m.config()
	if !ok {
		return m, nil
	}
	return m.start(opAction, func() tea.Msg { return appliedMsg{errResult{fn(c)}, done, hint} })
}

func (m Model) applied(msg appliedMsg) (tea.Model, tea.Cmd, bool) {
	owner := m.op.owner
	m = m.finish().refreshFrame(owner, m.opts.Config)
	if msg.err != nil {
		return m.fail(firstLine(msg.err), msg.hint), nil, true
	}
	return m.note("%s", msg.done), nil, true
}

func (m Model) refresh(c ConfigBackend) Model {
	return m.refreshFrame(m.topID(), c)
}

func (m Model) refreshFrame(id int, c ConfigBackend) Model {
	i := m.index(id)
	if i < 0 {
		return m
	}
	if s, ok := m.stack[i].screen.(settingsScreen); ok {
		m.stack[i].screen = s.reload(c)
	}
	return m
}
