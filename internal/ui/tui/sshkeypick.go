package tui

import (
	"path/filepath"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/elliot40404/creds/internal/app"
	"github.com/elliot40404/creds/internal/sshdir"
)

const keyPickChrome = 4

type (
	sshPickMsg   struct{ rels []string }
	sshUnskipMsg struct{ path string }
	sshKeyPicker struct {
		rels   []string
		on     []bool
		cursor int
	}
)

func (msg sshPickMsg) screen() Screen { return newKeyPicker(msg.rels) }

func (msg sshUnskipMsg) apply(m Model) (tea.Model, tea.Cmd) {
	b := m.opts.Backend
	return m.afterSync().start(opAction, func() tea.Msg {
		res, err := b.SSHAddKey(msg.path, "")
		return sshRan(b, false, res.Files, err)
	})
}

func asksKey(f app.SSHFileStatus) bool {
	return f.Change == app.SSHLocalNew && f.Unused && f.Kind == sshdir.KindPrivateKey && !f.Skipped
}

func (s sshScreen) push() tea.Cmd {
	var rels []string
	for _, f := range s.status.Files {
		if asksKey(f) {
			rels = append(rels, f.Rel)
		}
	}
	if len(rels) > 0 {
		return send(sshPickMsg{rels})
	}
	return send(sshPushMsg{})
}

func pushSkipped(f app.SSHFileStatus, dir string) tea.Cmd {
	if f.Next() != app.SSHSkippedKey {
		return nil
	}
	return send(sshUnskipMsg{filepath.Join(dir, filepath.FromSlash(f.Rel))})
}

func newKeyPicker(rels []string) sshKeyPicker {
	return sshKeyPicker{rels: rels, on: make([]bool, len(rels))}
}

func (p sshKeyPicker) keys() []keyHelp {
	return []keyHelp{
		{"space", "pick or unpick"},
		{"enter", "push"},
		{"esc", "cancel"},
		{"a", "all or none"},
		{"up/down j/k", "move"},
	}
}

func (p sshKeyPicker) Update(msg tea.Msg) (Screen, tea.Cmd) {
	key, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return p, nil
	}
	if dy := navDelta(key); dy != 0 {
		p.cursor = moveCursor(p.cursor, dy, len(p.rels))
		return p, nil
	}
	switch key.String() {
	case "esc":
		return p, send(popMsg{})
	case "space":
		p.on = slices.Clone(p.on)
		p.on[p.cursor] = !p.on[p.cursor]
	case "a":
		p.on = slices.Repeat([]bool{slices.Contains(p.on, false)}, len(p.on))
	case "enter":
		return p, send(confirmedMsg{then: sshPushMsg{keys: p.picked()}})
	}
	return p, nil
}

func (p sshKeyPicker) picked() map[string]bool {
	keys := make(map[string]bool, len(p.rels))
	for i, rel := range p.rels {
		keys[rel] = p.on[i]
	}
	return keys
}

func (p sshKeyPicker) View(width, height int) string {
	var b strings.Builder
	b.WriteString(line(th.title.Render("unused keys") + "  " + th.dim.Render("no host in the ssh config uses them")))
	b.WriteString(line(fit(width, th.text.Render("pick the keys to push, the rest are skipped and not asked again"))))
	b.WriteString(line(""))
	start, end := window(p.cursor, height-keyPickChrome, len(p.rels))
	for i := start; i < end; i++ {
		box := "[ ] "
		if p.on[i] {
			box = "[x] "
		}
		b.WriteString(pickRow(width, i == p.cursor, func(t theme) string { return t.text.Render(box + p.rels[i]) }))
	}
	b.WriteString(line(th.dim.Render("u on a skipped key pushes it later")))
	return b.String()
}
