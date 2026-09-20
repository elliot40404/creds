package tui

import (
	"image/color"
	"io"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/elliot40404/creds/internal/safetext"
)

var (
	colorBase    = lipgloss.Color("#121212")
	colorSurface = lipgloss.Color("#333333")
	colorText    = lipgloss.Color("#BEBEBE")
	colorBright  = lipgloss.Color("#EAEAEA")
	colorDim     = lipgloss.Color("#8A8A8D")
	colorAmber   = lipgloss.Color("#FFC107")
	colorOrange  = lipgloss.Color("#F59E0B")
	colorBadge   = lipgloss.Color("#E68E0D")
	colorDanger  = lipgloss.Color("#B91C1C")
	colorError   = lipgloss.Color("#D35F5F")
)

const (
	markerOn  = "▌ "
	markerOff = "  "
	ellipsis  = "…"
)

type theme struct {
	title  style
	text   style
	dim    style
	marker style
	match  style
	badge  style
	key    style
	label  style
	masked style
	err    style
	hint   style
	ok     style
	warn   style
	fail   style
	danger style
	brand  style
	border style
}

func fg(c color.Color) lipgloss.Style {
	return lipgloss.NewStyle().Foreground(c)
}

var th = theme{
	title:  style{fg(colorBright).Bold(true)},
	text:   style{fg(colorText)},
	dim:    style{fg(colorDim)},
	marker: style{fg(colorAmber).Bold(true)},
	match:  style{fg(colorAmber).Bold(true)},
	badge:  style{fg(colorBadge)},
	key:    style{fg(colorAmber).Bold(true)},
	label:  style{fg(colorDim)},
	masked: style{fg(colorDim)},
	err:    style{fg(colorError).Bold(true)},
	hint:   style{fg(colorDim)},
	ok:     style{fg(colorBright)},
	warn:   style{fg(colorOrange).Bold(true)},
	fail:   style{fg(colorError).Bold(true)},
	danger: style{fg(colorBright).Background(colorDanger).Bold(true).Padding(0, 1)},
	brand:  style{fg(colorBase).Background(colorAmber).Bold(true).Padding(0, 1)},
	border: style{lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(colorSurface).Padding(0, 1)},
}

var canvasOn, canvasOff = canvasSeqs()

func canvasSeqs() (string, string) {
	s := lipgloss.NewStyle().Background(colorBase).Render(" ")
	i := strings.Index(s, " ")
	return s[:i], s[i+1:]
}

type style struct {
	lipgloss.Style
}

func (s style) Render(strs ...string) string {
	return s.Style.Render(safetext.Line(strings.Join(strs, " ")))
}

func (t theme) row(focused bool) theme {
	if !focused {
		return t
	}
	on := func(s style) style { return style{s.Background(colorSurface)} }
	t.title, t.text, t.dim = on(t.title), on(style{t.text.Foreground(colorBright)}), on(t.dim)
	t.match, t.badge, t.masked, t.label = on(t.match), on(t.badge), on(t.masked), on(t.label)
	return t
}

func (t theme) marked(focused bool) string {
	if focused {
		return t.marker.Background(colorSurface).Render(markerOn)
	}
	return markerOff
}

func hasColor(out io.Writer, env []string) bool {
	return colorprofile.Detect(out, env) > colorprofile.ASCII
}
