package tui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/elliot40404/creds/internal/app"
)

type reqKind int

const (
	reqPassword reqKind = iota
	reqInput
	reqSelect
	reqConfirm
	reqShow
	reqChecks
)

type setupReq struct {
	kind    reqKind
	prompt  string
	def     string
	text    string
	keep    string
	options []string
	cur     int
	checks  []app.Check
	reply   chan setupAns
}

type setupAns struct {
	text     string
	idx      int
	ok       bool
	canceled bool
}

type (
	setupAskMsg  struct{ req *setupReq }
	setupNoteMsg struct {
		msg  string
		warn bool
	}
	setupDoneMsg struct{ err error }
)

func screenFor(req *setupReq) Screen {
	switch req.kind {
	case reqPassword:
		return setupInput{prompt: req.prompt, keep: req.keep, secret: true, input: newInput("", true)}
	case reqInput:
		return setupInput{prompt: req.prompt, def: req.def, keep: req.keep, input: newInput("", false)}
	case reqSelect:
		return setupSelect{prompt: req.prompt, options: req.options, cursor: moveCursor(req.cur, 0, len(req.options))}
	case reqConfirm:
		return setupConfirm{prompt: req.prompt}
	case reqShow:
		return setupShow{title: req.prompt, text: req.text}
	case reqChecks:
		return setupChecks{checks: req.checks}
	}
	return setupWork{}
}

type setupInput struct {
	prompt string
	def    string
	keep   string
	secret bool
	input  textInput
	errMsg string
}

func (s setupInput) typing() bool {
	return true
}

func (s setupInput) keys() []keyHelp {
	return []keyHelp{{"enter", "continue"}, {"ctrl+u", "clear"}, {"esc", "cancel setup"}}
}

func (s setupInput) Update(msg tea.Msg) (Screen, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.PasteMsg:
		s.input.update(msg)
	case tea.KeyPressMsg:
		switch msg.String() {
		case "esc":
			return s, send(setupAns{canceled: true})
		case "enter":
			return s.submit()
		}
		if s.input.update(msg) {
			s.errMsg = ""
		}
	}
	return s, nil
}

func (s setupInput) submit() (Screen, tea.Cmd) {
	value := s.input.String()
	if value == "" {
		value = s.def
	}
	if value == "" {
		s.errMsg = "this cannot be empty. type a value, or press esc to cancel setup"
		return s, nil
	}
	return s, send(setupAns{text: value})
}

func (s setupInput) View(width, _ int) string {
	var out strings.Builder
	out.WriteString(line(th.title.Render(s.prompt)) + line(""))
	if s.keep != "" {
		for _, l := range wrapWords(s.keep, width) {
			out.WriteString(line(th.text.Render(l)))
		}
		out.WriteString(line(""))
	}
	out.WriteString(line(th.key.Render("›") + " " + s.input.view(true)))
	if s.def != "" {
		out.WriteString(line(th.dim.Render("  blank keeps " + s.def)))
	}
	if s.errMsg != "" {
		out.WriteString(line("") + line(th.err.Render(s.errMsg)))
	}
	return out.String()
}

type setupSelect struct {
	prompt  string
	options []string
	cursor  int
}

func (s setupSelect) keys() []keyHelp {
	return []keyHelp{{"up/down j/k", "move"}, {"1-9", "pick that one"}, {"enter", "pick"}, {"esc", "cancel setup"}}
}

func (s setupSelect) Update(msg tea.Msg) (Screen, tea.Cmd) {
	key, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return s, nil
	}
	if i, ok := pickNumber(key, len(s.options)); ok {
		return s, send(setupAns{idx: i, text: s.options[i], ok: true})
	}
	if dy := navDelta(key); dy != 0 {
		s.cursor = moveCursor(s.cursor, dy, len(s.options))
		return s, nil
	}
	switch key.String() {
	case "enter":
		return s, send(setupAns{idx: s.cursor, text: s.options[s.cursor], ok: true})
	case "esc":
		return s, send(setupAns{canceled: true})
	}
	return s, nil
}

func (s setupSelect) View(width, height int) string {
	return line(th.title.Render(s.prompt)) + line("") + choices(width, height-2, s.options, s.cursor)
}

type setupConfirm struct {
	prompt string
}

func (s setupConfirm) keys() []keyHelp {
	return []keyHelp{{"y", "yes"}, {"n esc", "no"}}
}

func (s setupConfirm) Update(msg tea.Msg) (Screen, tea.Cmd) {
	key, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return s, nil
	}
	switch key.String() {
	case "y":
		return s, send(setupAns{ok: true})
	case "n", "esc", "enter":
		return s, send(setupAns{})
	}
	return s, nil
}

func (s setupConfirm) View(int, int) string {
	return line(th.title.Render(s.prompt)) + line("") +
		line(th.key.Render("y")+th.label.Render(" yes   ")+th.key.Render("n")+th.label.Render(" no"))
}

type setupShow struct {
	title string
	text  string
}

func (s setupShow) keys() []keyHelp {
	return []keyHelp{{"enter", "continue"}}
}

func (s setupShow) Update(msg tea.Msg) (Screen, tea.Cmd) {
	return s, continueKey(msg)
}

func continueKey(msg tea.Msg) tea.Cmd {
	key, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return nil
	}
	switch key.String() {
	case "enter":
		return send(setupAns{ok: true})
	case "esc":
		return send(setupAns{canceled: true})
	}
	return nil
}

func (s setupShow) View(width, _ int) string {
	var out strings.Builder
	out.WriteString(line(th.title.Render(s.title)) + line(""))
	for _, l := range wrapWords(s.text, width) {
		out.WriteString(line(th.text.Render(l)))
	}
	out.WriteString(line(""))
	for _, l := range wrapWords("write it down now, it is shown once", width) {
		out.WriteString(line(th.dim.Render(l)))
	}
	return out.String()
}

type setupChecks struct {
	checks []app.Check
}

func (s setupChecks) keys() []keyHelp {
	return []keyHelp{{"enter", "continue"}, {"esc", "cancel setup"}}
}

func (s setupChecks) Update(msg tea.Msg) (Screen, tea.Cmd) {
	return s, continueKey(msg)
}

func (s setupChecks) View(width, _ int) string {
	var out strings.Builder
	out.WriteString(line(th.title.Render("Preflight")) + line(""))
	col := s.labelWidth()
	for _, c := range s.checks {
		state := th.ok.Render("ok  ")
		if !c.OK {
			state = th.warn.Render("fix ")
		}
		out.WriteString(line(fit(width, state+th.label.Render(padRight(c.Name, col))+th.text.Render(c.Message))))
		if !c.OK && c.Fix != "" {
			out.WriteString(line(fit(width, th.hint.Render("    "+padRight("", col)+c.Fix))))
		}
	}
	return out.String()
}

func (s setupChecks) labelWidth() int {
	return widest(s.checks, func(c app.Check) string { return c.Name }) + 2
}

type setupWork struct {
	what string
}

func (s setupWork) typing() bool {
	return true
}

func (s setupWork) keys() []keyHelp {
	return []keyHelp{{"ctrl+c", "cancel setup"}}
}

func (s setupWork) Update(tea.Msg) (Screen, tea.Cmd) {
	return s, nil
}

func (s setupWork) View(int, int) string {
	what := s.what
	if what == "" {
		what = "working"
	}
	return line(th.dim.Render(what))
}
