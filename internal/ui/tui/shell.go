package tui

import (
	"fmt"
	"io"
	"os"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/elliot40404/creds/internal/app"
	"github.com/elliot40404/creds/internal/config"
	"github.com/elliot40404/creds/internal/render"
	"github.com/elliot40404/creds/internal/safetext"
	"github.com/elliot40404/creds/internal/search"
	"github.com/elliot40404/creds/internal/vault"
)

type Backend interface {
	List() ([]search.Summary, error)
	Get(path string) (vault.Entry, error)
	Add(e vault.Entry) (vault.Entry, error)
	Update(path string, e vault.Entry) (vault.Entry, error)
	Delete(path string) error
	Formats(path string) ([]string, error)
	ShellVaries(path, format string) (bool, error)
	RenderShell() render.Shell
	Sync() (string, error)
	SyncStatus() (app.SyncStatus, error)
	Lock() error
	Warnings() []string
}

type Copier func(path, field, as string, sh render.Shell) (string, error)

type Layout struct {
	Inline    bool
	AltScreen bool
	Height    int
}

func DefaultLayout() Layout {
	return Layout{AltScreen: true, Height: config.DefaultHeight}
}

func (l Layout) rows(h int) int {
	if !l.Inline {
		return h
	}
	return min(h, max(l.Height, config.MinHeight))
}

type Options struct {
	Backend   Backend
	Config    ConfigBackend
	Copy      Copier
	Hooks     Hooks
	Unlock    func(password string) error
	VaultPath string
	Color     bool
	Layout    Layout
	Now       func() time.Time
}

type Model struct {
	opts        Options
	stack       []frame
	nextID      int
	op          op
	status      app.SyncStatus
	stErr       error
	flash       string
	hint        string
	failed      bool
	editing     *vault.Entry
	help        bool
	watching    bool
	quit        bool
	printOnQuit string
	width       int
	height      int
}

func New(opts Options) Model {
	if opts.Now == nil {
		opts.Now = time.Now
	}
	if opts.Layout == (Layout{}) {
		opts.Layout = DefaultLayout()
	}
	return Model{opts: opts, stack: []frame{{id: 1, screen: newList(nil)}}, nextID: 1, width: 80, height: 24}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(m.loadItems(), m.loadStatus(), m.syncIfDue())
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, m.opts.Layout.rows(msg.Height)
		return m.resize(), nil
	case tea.KeyPressMsg:
		return m.key(msg)
	}
	if next, cmd, ok := m.handle(msg); ok {
		return next, cmd
	}
	return m.forward(msg)
}

func (m Model) key(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "ctrl+c" {
		return m.stop()
	}
	if m.help {
		m.help = false
		return m, nil
	}
	m.flash, m.hint, m.failed = "", "", false
	if _, ok := m.top().(helper); ok && !typing(m.top()) && msg.String() == "?" {
		m.help = true
		return m, nil
	}
	return m.forward(msg)
}

func (m Model) forward(msg tea.Msg) (tea.Model, tea.Cmd) {
	return m.sendTo(m.topID(), msg)
}

func (m Model) resize() Model {
	size := tea.WindowSizeMsg{Width: innerWidth(m.width), Height: m.bodyHeight()}
	for i, f := range m.stack {
		m.stack[i].screen, _ = f.screen.Update(size)
	}
	return m
}

func (m Model) top() Screen {
	return m.stack[len(m.stack)-1].screen
}

func (m Model) push(s Screen) Model {
	m.nextID++
	m.stack = append(m.stack[:len(m.stack):len(m.stack)], frame{id: m.nextID, screen: s})
	return m.resize()
}

func (m Model) pop() (tea.Model, tea.Cmd) {
	if len(m.stack) == 1 {
		return m.stop()
	}
	m.stack = m.stack[:len(m.stack)-1]
	return m, nil
}

func (m Model) stop() (tea.Model, tea.Cmd) {
	m.quit = true
	return m, tea.Quit
}

func (m Model) note(format string, args ...any) Model {
	m.flash, m.hint, m.failed = fmt.Sprintf(format, args...), "", false
	return m
}

func (m Model) fail(err error, fix string) Model {
	m.flash, m.hint, m.failed = err.Error(), fix, true
	return m
}

func (m Model) View() tea.View {
	return newView(m.quit, m.opts.Layout.AltScreen, m.frame)
}

func Run(opts Options, in io.Reader, out io.Writer) error {
	opts.Color = hasColor(out, os.Environ())
	final, err := tea.NewProgram(New(opts), tea.WithInput(in), tea.WithOutput(out)).Run()
	if err != nil {
		return fmt.Errorf("tui: %w", err)
	}
	if m, ok := final.(Model); ok && m.printOnQuit != "" {
		_, err = io.WriteString(out, safetext.Text(m.printOnQuit)+"\n")
	}
	return err
}
