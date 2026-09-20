package tui

import (
	"fmt"
	"strings"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
)

func widest[T any](xs []T, text func(T) string) int {
	w := 0
	for _, x := range xs {
		w = max(w, utf8.RuneCountInString(text(x)))
	}
	return w
}

func innerWidth(width int) int {
	return max(width-panelInset, 1)
}

func tooSmall(width, height int) (string, bool) {
	if width >= minWidth && height >= minHeight {
		return "", false
	}
	return fit(width, th.warn.Render(fmt.Sprintf("window too small, make it at least %dx%d", minWidth, minHeight))), true
}

func panelLines(width, rows int, body string) []string {
	lines := strings.Split(strings.TrimSuffix(body, "\n"), "\n")
	lines = lines[:min(len(lines), rows)]
	for i, l := range lines {
		lines[i] = fit(innerWidth(width), l)
	}
	return lines
}

func panel(width, height int, lines []string) string {
	return th.border.Width(width).Height(height).Render(strings.Join(lines, "\n"))
}

func newView(quit, alt bool, frame func() string) tea.View {
	v := tea.NewView("")
	if !quit {
		v = tea.NewView(frame())
	}
	v.AltScreen = alt
	return v
}
