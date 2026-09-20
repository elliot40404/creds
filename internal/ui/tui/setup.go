package tui

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/elliot40404/creds/internal/app"
	"github.com/elliot40404/creds/internal/fail"
)

var errSetupCanceled = errors.New("setup canceled")

const (
	logRows         = 5
	setupHeaderRows = 2
	setupFooterRows = 1
)

type SetupOptions struct {
	Setup *app.Setup
	Color bool
}

type setupNote struct {
	text string
	warn bool
}

type setupModel struct {
	opts   SetupOptions
	screen Screen
	req    *setupReq
	log    []setupNote
	shown  string
	err    error
	done   bool
	quit   bool
	width  int
	height int
	ch     chan tea.Msg
	stop   chan struct{}
	ctx    context.Context
	cancel context.CancelFunc
}

func newSetup(opts SetupOptions) setupModel {
	ctx, cancel := context.WithCancel(context.Background())
	return setupModel{
		opts:   opts,
		screen: setupWork{"starting"},
		width:  80,
		height: 24,
		ch:     make(chan tea.Msg),
		stop:   make(chan struct{}),
		ctx:    ctx,
		cancel: cancel,
	}
}

func RunSetup(opts SetupOptions, in io.Reader, out io.Writer) error {
	opts.Color = hasColor(out, os.Environ())
	m := newSetup(opts)
	defer m.cancel()
	defer close(m.stop)
	final, err := tea.NewProgram(m, tea.WithInput(in), tea.WithOutput(out)).Run()
	if err != nil {
		return fmt.Errorf("tui: %w", err)
	}
	done, ok := final.(setupModel)
	if !ok || errors.Is(done.err, errSetupCanceled) {
		return nil
	}
	return done.err
}

func (m setupModel) Init() tea.Cmd {
	go m.drive(m.prompter())
	return m.wait()
}

func (m setupModel) prompter() chanPrompter {
	return chanPrompter{out: m.ch, stop: m.stop, ctx: m.ctx}
}

func (m setupModel) drive(p chanPrompter) {
	m.opts.Setup.Service.Prompter = p
	err := SetupFlow(m.ctx, m.opts.Setup, p)
	p.send(setupDoneMsg{err})
}

func (m setupModel) wait() tea.Cmd {
	return func() tea.Msg {
		select {
		case msg := <-m.ch:
			return msg
		case <-m.stop:
			return setupDoneMsg{errSetupCanceled}
		}
	}
}

func (m setupModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.screen, _ = m.screen.Update(m.size())
		return m, nil
	case setupAskMsg:
		return m.ask(msg.req)
	case setupNoteMsg:
		m.log = append(m.log, setupNote{text: noteText(msg), warn: msg.warn})
		return m, m.wait()
	case setupDoneMsg:
		return m.finish(msg.err)
	case setupAns:
		return m.answer(msg)
	case tea.KeyPressMsg:
		return m.key(msg)
	}
	return m.forward(msg)
}

func (m setupModel) ask(req *setupReq) (tea.Model, tea.Cmd) {
	m.req = req
	switch req.kind {
	case reqShow:
		m.shown = req.text
	case reqInput, reqPassword:
		req.keep = m.shown
	case reqSelect, reqConfirm, reqChecks:
		m.shown = ""
	}
	m.screen, _ = screenFor(req).Update(m.size())
	return m, m.wait()
}

func (m setupModel) dropWarnings() []setupNote {
	kept := make([]setupNote, 0, len(m.log))
	for _, n := range m.log {
		if !n.warn {
			kept = append(kept, n)
		}
	}
	return kept
}

func noteText(msg setupNoteMsg) string {
	if msg.warn {
		return "warning: " + msg.msg
	}
	return msg.msg
}

func (m setupModel) size() tea.WindowSizeMsg {
	return tea.WindowSizeMsg{Width: innerWidth(m.width), Height: m.panelHeight()}
}

func (m setupModel) key(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "ctrl+c" {
		return m.answer(setupAns{canceled: true})
	}
	if m.done {
		m.quit = true
		return m, tea.Quit
	}
	return m.forward(msg)
}

func (m setupModel) forward(msg tea.Msg) (tea.Model, tea.Cmd) {
	next, cmd := m.screen.Update(msg)
	m.screen = next
	return m, cmd
}

func (m setupModel) answer(ans setupAns) (tea.Model, tea.Cmd) {
	if m.done {
		m.quit = true
		return m, tea.Quit
	}
	if m.req != nil {
		m.req.reply <- ans
		m.req = nil
	}
	if ans.canceled {
		m.cancel()
	}
	m.log = m.dropWarnings()
	m.screen = setupWork{"working"}
	if ans.canceled {
		m.screen = setupWork{"canceling"}
	}
	return m, nil
}

func (m setupModel) finish(err error) (tea.Model, tea.Cmd) {
	m.done, m.err, m.req = true, err, nil
	if errors.Is(err, errSetupCanceled) {
		m.quit = true
		return m, tea.Quit
	}
	m.screen = setupEnd{err: err, steps: m.steps()}
	return m, nil
}

func (m setupModel) steps() []string {
	if m.err != nil {
		return nil
	}
	return m.opts.Setup.NextSteps()
}

func (m setupModel) View() tea.View {
	return newView(m.quit, true, m.frame)
}

func (m setupModel) panelHeight() int {
	return max(m.height-setupHeaderRows-setupFooterRows-panelChrome-max(m.logHeight(), 1), 1)
}

func (m setupModel) frame() string {
	if small, ok := tooSmall(m.width, m.height); ok {
		return small
	}
	lines := panelLines(m.width, m.panelHeight(), m.screen.View(innerWidth(m.width), m.panelHeight()))
	return paint(m.opts.Color, m.width,
		[]string{m.header(), "", panel(m.width, len(lines)+panelChrome, lines), m.logView(), m.footer()})
}

func (m setupModel) header() string {
	return fit(m.width, th.brand.Render("creds")+"  "+th.title.Render("first run setup"))
}

func (m setupModel) logView() string {
	var b strings.Builder
	for _, n := range m.log[len(m.log)-m.logHeight():] {
		style := th.dim
		if n.warn {
			style = th.warn
		}
		b.WriteString(line(fit(m.width, style.Render(n.text))))
	}
	return strings.TrimSuffix(b.String(), "\n")
}

func (m setupModel) logHeight() int {
	return min(len(m.log), logRows)
}

func (m setupModel) footer() string {
	h, ok := m.screen.(helper)
	if !ok {
		return ""
	}
	return hintBar(m.width, h.keys())
}

type setupEnd struct {
	err   error
	steps []string
}

func (s setupEnd) keys() []keyHelp {
	return []keyHelp{{"any key", "close"}}
}

func (s setupEnd) Update(tea.Msg) (Screen, tea.Cmd) {
	return s, nil
}

func (s setupEnd) View(width, _ int) string {
	if s.err != nil {
		return s.failView(width)
	}
	var out strings.Builder
	out.WriteString(line(th.title.Render("Setup done")) + line(""))
	for _, step := range s.steps {
		out.WriteString(line(th.key.Render("›") + " " + th.text.Render(step)))
	}
	return out.String()
}

func (s setupEnd) failView(width int) string {
	e := fail.Classify(s.err)
	var out strings.Builder
	out.WriteString(line(th.err.Render("Setup did not finish")) + line(""))
	for _, l := range wrapWords(firstLine(e).Error(), width) {
		out.WriteString(line(th.text.Render(l)))
	}
	if e.Hint != "" {
		out.WriteString(line("") + line(th.hint.Render(e.Hint)))
	}
	return out.String()
}

func wrapWords(s string, width int) []string {
	width = max(width, 1)
	var out []string
	current := ""
	for word := range strings.FieldsSeq(s) {
		switch {
		case current == "":
			current = word
		case ansi.StringWidth(current+" "+word) <= width:
			current += " " + word
		default:
			out = append(out, current)
			current = word
		}
	}
	if current != "" {
		out = append(out, current)
	}
	return out
}
