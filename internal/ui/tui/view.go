package tui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/charmbracelet/x/ansi"
	"github.com/elliot40404/creds/internal/app"
	"github.com/elliot40404/creds/internal/fail"
	"github.com/elliot40404/creds/internal/safetext"
)

const (
	headerRows  = 2
	footerRows  = 2
	panelChrome = 2
	panelInset  = 4
	minWidth    = 30
	minHeight   = 10
)

type keyHelp struct {
	key  string
	desc string
}

type helper interface {
	keys() []keyHelp
}

type typer interface {
	typing() bool
}

func typing(s Screen) bool {
	t, ok := s.(typer)
	return ok && t.typing()
}

func fit(width int, s string) string {
	return ansi.Truncate(s, max(width, 1), ellipsis)
}

func fill(width int, s string) string {
	if n := width - ansi.StringWidth(s); n > 0 {
		return s + strings.Repeat(" ", n)
	}
	return s
}

func padRight(s string, n int) string {
	if pad := n - len([]rune(s)); pad > 0 {
		return s + strings.Repeat(" ", pad)
	}
	return s
}

func line(s string) string {
	return s + "\n"
}

func spread(width int, left, right string) string {
	gap := width - ansi.StringWidth(left) - ansi.StringWidth(right)
	if gap < 1 {
		return left
	}
	return left + strings.Repeat(" ", gap) + right
}

func pickRow(width int, focused bool, build func(t theme) string) string {
	t := th.row(focused)
	s := fit(width-len(markerOff), build(t))
	if focused {
		s += t.text.Render(strings.Repeat(" ", max(width-len(markerOff)-ansi.StringWidth(s), 0)))
	}
	return th.marked(focused) + s + "\n"
}

func navDelta(key tea.KeyPressMsg) int {
	switch key.String() {
	case "up", "k":
		return -1
	case "down", "j":
		return 1
	}
	return 0
}

func moveCursor(cursor, delta, n int) int {
	return min(max(cursor+delta, 0), max(n-1, 0))
}

func window(cursor, rows, n int) (start, end int) {
	rows = max(rows, 1)
	start = max(cursor-rows+1, 0)
	return start, min(n, start+rows)
}

const maxPickNumber = 9

func pickPrefix(i int) string {
	if i < maxPickNumber {
		return strconv.Itoa(i+1) + " "
	}
	return "  "
}

func choices(width, rows int, names []string, cursor int) string {
	var b strings.Builder
	start, end := window(cursor, rows, len(names))
	for i := start; i < end; i++ {
		b.WriteString(pickRow(width, i == cursor, func(t theme) string {
			return th.key.Render(pickPrefix(i)) + t.text.Render(names[i])
		}))
	}
	return b.String()
}

const shiftDigits = "!@#$%^&*("

func pickShiftNumber(key tea.KeyPressMsg, n int) (int, bool) {
	i := -1
	switch {
	case key.Mod&tea.ModShift != 0 && key.BaseCode >= '1' && key.BaseCode <= '9':
		i = int(key.BaseCode - '1')
	case len(key.Text) == 1:
		i = strings.IndexByte(shiftDigits, key.Text[0])
	}
	if i < 0 || i >= n {
		return 0, false
	}
	return i, true
}

func pickNumber(key tea.KeyPressMsg, n int) (int, bool) {
	s := key.String()
	if len(s) != 1 || s[0] < '1' || s[0] > '9' {
		return 0, false
	}
	i := int(s[0] - '1')
	if i >= n {
		return 0, false
	}
	return i, true
}

func errLine(width int, msg, fix string) string {
	s := th.err.Render(msg)
	if fix != "" {
		s += th.hint.Render(". " + fix)
	}
	return fit(width, s)
}

func (m Model) bodyHeight() int {
	return max(m.height-headerRows-footerRows-panelChrome, 1)
}

func (m Model) frame() string {
	if small, ok := tooSmall(m.width, m.height); ok {
		return small
	}
	body := m.top().View(innerWidth(m.width), m.bodyHeight())
	if m.help {
		body = helpView(m.top())
	}
	return paint(m.opts.Color, m.width,
		[]string{m.header(), m.statusView(), panel(m.width, m.bodyHeight()+panelChrome, panelLines(m.width, m.bodyHeight(), body)), m.flashView(), m.footer()})
}

func paint(color bool, width int, parts []string) string {
	joined := strings.Join(parts, "\n")
	if !color {
		return joined
	}
	lines := strings.Split(joined, "\n")
	for i, l := range lines {
		lines[i] = canvasOn + strings.ReplaceAll(fill(width, l), canvasOff, canvasOff+canvasOn) + canvasOff
	}
	return strings.Join(lines, "\n")
}

func (m Model) header() string {
	st := m.status
	remote := "none"
	parts := []string{th.brand.Render("creds")}
	if st.Remote != "" {
		remote = safetext.Remote(st.Remote)
		parts = append(parts, m.syncBadge(st))
	}
	if n := len(st.Conflicts); n > 0 {
		parts = append(parts, th.warn.Render(fmt.Sprintf("▲ conflicts %d", n)))
	}
	parts = append(parts,
		th.label.Render("remote ")+th.text.Render(remote),
		th.label.Render("vault ")+th.text.Render(m.opts.VaultPath),
	)
	return fit(m.width, strings.Join(parts, "  "))
}

func (m Model) statusView() string {
	switch {
	case m.stErr != nil:
		return errLine(m.width, "cannot read sync status: "+firstLine(m.stErr).Error(), "run creds status")
	case m.status.LastError != "":
		msg, hint := fail.SyncText(m.status.LastError)
		if hint != "" {
			hint += ", or "
		}
		return errLine(m.width, "last sync failed: "+msg, hint+"press s to retry")
	}
	return ""
}

func (m Model) flashView() string {
	switch {
	case m.flash == "":
		return ""
	case m.failed:
		return errLine(m.width, m.flash, m.hint)
	}
	return fit(m.width, th.ok.Render("✓ "+m.flash))
}

func (m Model) footer() string {
	if m.help {
		return hintBar(m.width, []keyHelp{{"any key", "close help"}})
	}
	h, ok := m.top().(helper)
	if !ok {
		return ""
	}
	keys := h.keys()
	if !typing(m.top()) {
		keys = append([]keyHelp{{"?", "help"}}, keys...)
	}
	return hintBar(m.width, keys)
}

func hintBar(width int, keys []keyHelp) string {
	var b strings.Builder
	used := 0
	for _, k := range keys {
		item := th.key.Render(k.key) + " " + th.label.Render(k.desc)
		w := ansi.StringWidth(item)
		if used > 0 {
			w += 2
		}
		if used+w > width {
			break
		}
		if used > 0 {
			b.WriteString("  ")
		}
		b.WriteString(item)
		used += w
	}
	return b.String()
}

func (m Model) ago(t time.Time) string {
	if t.IsZero() {
		return "never"
	}
	d := m.opts.Now().Sub(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 48*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	}
	return fmt.Sprintf("%dd ago", int(d.Hours()/24))
}

func helpView(s Screen) string {
	h, ok := s.(helper)
	if !ok {
		return ""
	}
	keys := h.keys()
	w := widest(keys, func(k keyHelp) string { return k.key })
	var b strings.Builder
	b.WriteString(line(th.title.Render("Keys")))
	b.WriteString(line(""))
	for _, k := range keys {
		b.WriteString(line(th.key.Render(padRight(k.key, w)) + "  " + th.text.Render(k.desc)))
	}
	b.WriteString(line(th.hint.Render("any key closes this help, ctrl+c quits")))
	return b.String()
}

func (m Model) syncBadge(st app.SyncStatus) string {
	if m.op.syncing() {
		return th.dim.Render("◌ syncing")
	}
	state := th.ok
	if st.LastError != "" || m.stErr != nil {
		state = th.fail
	}
	return state.Render("● synced " + m.ago(st.LastSync))
}
