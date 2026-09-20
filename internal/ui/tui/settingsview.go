package tui

import (
	"strings"

	"github.com/elliot40404/creds/internal/app"
	"github.com/elliot40404/creds/internal/config"
)

const minSettingRows = 3

func (s settingsScreen) previewLine() string {
	if s.previewValue() == "" {
		return ""
	}
	if s.outErr != nil {
		return th.err.Render(firstLine(s.outErr).Error())
	}
	return th.dim.Render("sample  ") + th.text.Render(s.out)
}

func (s settingsScreen) View(width, height int) string {
	var b strings.Builder
	b.WriteString(line(th.title.Render("Settings") + "  " + th.dim.Render("saved to config.toml, checked before saving")))
	b.WriteString(line(""))
	if s.adding {
		b.WriteString(line(th.label.Render("new key  ") + s.input.view(true)))
		b.WriteString(line(th.hint.Render("use render.formats.<engine>.<name>, for example render.formats.postgres.psql")))
		return b.String()
	}
	tail := s.tail(width)
	block, gap := s.pathBlock(width), line("")
	room := height - 4 - strings.Count(tail, "\n")
	if room-strings.Count(block, "\n") < min(len(s.rows), minSettingRows) {
		block, gap = s.pathSummary(width), ""
		room += 2
	}
	b.WriteString(block)
	b.WriteString(gap)
	b.WriteString(s.list(width, max(room-strings.Count(block, "\n"), 1)))
	b.WriteString(gap)
	b.WriteString(tail)
	return b.String()
}

func (s settingsScreen) tail(width int) string {
	var b strings.Builder
	if doc := s.current().doc; doc != "" {
		b.WriteString(line(fit(width, th.hint.Render(doc))))
	}
	if h := s.editHint(); h != "" {
		b.WriteString(line(fit(width, th.hint.Render(h))))
	}
	if p := s.previewLine(); p != "" {
		b.WriteString(line(fit(width, p)))
	}
	return b.String()
}

func (s settingsScreen) pathHead() string {
	head := th.label.Render("Files")
	if s.paths.HomeEnv {
		head += "  " + th.dim.Render(config.HomeEnv+" is set")
	}
	return head
}

func (s settingsScreen) pathSummary(width int) string {
	return line(fit(width, s.pathHead()+"  "+th.dim.Render("hidden, make the window taller to see them")))
}

func (s settingsScreen) pathBlock(width int) string {
	var b strings.Builder
	b.WriteString(line(fit(width, s.pathHead())))
	nameWidth := widest(s.paths.Paths, func(p app.PathInfo) string { return p.Name })
	pathWidth := widest(s.paths.Paths, func(p app.PathInfo) string { return p.Path })
	for _, p := range s.paths.Paths {
		row := th.dim.Render(padRight(p.Name, nameWidth)+"  ") + th.text.Render(padRight(p.Path, pathWidth))
		if note := pathNote(p); note != "" {
			row += th.dim.Render("  " + note)
		}
		b.WriteString(line(fit(width, row)))
	}
	return b.String()
}

func (s settingsScreen) list(width, rows int) string {
	var b strings.Builder
	keyWidth := widest(s.rows, func(r settingRow) string { return r.key })
	start, end := window(s.cursor, rows, len(s.rows))
	for i := start; i < end; i++ {
		r := s.rows[i]
		b.WriteString(pickRow(width, i == s.cursor, func(t theme) string {
			out := t.text.Render(padRight(r.key, keyWidth) + "  ")
			if i == s.cursor && s.editing {
				return out + s.input.view(true)
			}
			switch {
			case i == s.cursor && r.cycles():
				out += th.key.Render("‹ ") + t.text.Render(r.value) + th.key.Render(" ›")
			case r.value == "" && r.derived != "":
				return out + th.dim.Render(r.derived+"  ("+r.source+")")
			default:
				out += t.text.Render(r.value)
			}
			if r.kind == settingConfig && r.value != r.def && r.def != "" {
				out += th.dim.Render("  (default " + r.def + ")")
			}
			return out
		}))
	}
	return b.String()
}
