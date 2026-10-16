package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/elliot40404/creds/internal/app"
	"github.com/elliot40404/creds/internal/textdiff"
)

const sshDiffChrome = 2

type sshDiffScreen struct {
	file   app.SSHFileStatus
	lines  []string
	offset int
	rows   int
}

func newSSHDiff(f app.SSHFileStatus) sshDiffScreen {
	return sshDiffScreen{file: f, lines: diffLines(f)}
}

func diffLines(f app.SSHFileStatus) []string {
	if f.Change == app.SSHSame {
		return []string{th.dim.Render("same here and in the vault")}
	}
	d := f.Diff()
	if d.Key {
		return []string{th.warn.Render("key differs, content not shown")}
	}
	out := make([]string, 0, len(d.Lines)+1)
	for _, l := range d.Lines {
		out = append(out, diffLine(l))
	}
	if d.More > 0 {
		out = append(out, th.dim.Render(fmt.Sprintf("... %d more lines, run creds ssh pull in a terminal", d.More)))
	}
	return out
}

func diffLine(l textdiff.Line) string {
	switch l.Op {
	case textdiff.Gap:
		return th.dim.Render("...")
	case textdiff.Del:
		return th.fail.Render("- " + l.Text)
	case textdiff.Add:
		return th.ok.Render("+ " + l.Text)
	}
	return th.dim.Render("  " + l.Text)
}

func (s sshDiffScreen) keys() []keyHelp {
	return []keyHelp{{"up/down j/k", "scroll"}, {"esc q", "back"}}
}

func (s sshDiffScreen) Update(msg tea.Msg) (Screen, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		s.rows = max(msg.Height-sshDiffChrome, 1)
		s.offset = s.scroll(0)
	case tea.KeyPressMsg:
		if dy := navDelta(msg); dy != 0 {
			s.offset = s.scroll(dy)
		}
		if k := msg.String(); k == "esc" || k == "q" {
			return s, send(popMsg{})
		}
	}
	return s, nil
}

func (s sshDiffScreen) scroll(dy int) int {
	return moveCursor(s.offset, dy, max(len(s.lines)-s.rows, 0)+1)
}

func (s sshDiffScreen) View(_, height int) string {
	var b strings.Builder
	b.WriteString(line(th.title.Render(s.file.Rel) + "  " + th.dim.Render(s.file.Change.String()+", - here, + vault")))
	b.WriteString(line(""))
	rows := max(height-sshDiffChrome, 1)
	start := min(s.offset, len(s.lines))
	for _, l := range s.lines[start:min(start+rows, len(s.lines))] {
		b.WriteString(line(l))
	}
	return b.String()
}
