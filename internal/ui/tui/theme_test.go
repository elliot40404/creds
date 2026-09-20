package tui

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/x/ansi"
	"github.com/elliot40404/creds/internal/app"
	"github.com/elliot40404/creds/internal/vault"
)

var update = flag.Bool("update", false, "rewrite golden files")

func sized(t *testing.T, f *fakeBackend, w, h int) Model {
	t.Helper()
	return step(start(t, f, Hooks{}), tea.WindowSizeMsg{Width: w, Height: h})
}

func golden(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join("testdata", name+".golden")
	if *update {
		if err := os.MkdirAll("testdata", 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(got), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	want, err := fs.ReadFile(os.DirFS("testdata"), name+".golden")
	if err != nil {
		t.Fatal(err)
	}
	if got != strings.ReplaceAll(string(want), "\r\n", "\n") {
		t.Fatalf("%s mismatch, run go test -update\ngot:\n%s", name, got)
	}
}

func fitsScreen(t *testing.T, v string, w, h int) {
	t.Helper()
	lines := strings.Split(v, "\n")
	if len(lines) != h {
		t.Fatalf("%d lines, want %d", len(lines), h)
	}
	for i, l := range lines {
		if n := ansi.StringWidth(l); n > w {
			t.Fatalf("line %d is %d wide, want <= %d: %q", i, n, w, l)
		}
	}
}

func snapshotFake() *fakeBackend {
	f := newFake()
	f.status = app.SyncStatus{Remote: "https://example.com/vault.git", LastError: "network down", Conflicts: []string{"a"}}
	f.entries = append(f.entries, vault.Entry{
		Path: "work/very/long/path/that/keeps/going/and/going/forever/and/ever", Type: vault.TypeLogin,
		Username: "someone.with.a.long.name", Host: "a-very-long-hostname.internal.example.com",
	})
	return f
}

func TestViewSnapshots(t *testing.T) {
	for _, size := range [][2]int{{80, 24}, {120, 40}} {
		w, h := size[0], size[1]
		screens := map[string]Model{
			"list":   sized(t, snapshotFake(), w, h),
			"filter": press(sized(t, snapshotFake(), w, h), append([]tea.KeyPressMsg{ch('/')}, text("wm")...)...),
			"detail": press(sized(t, snapshotFake(), w, h), down, enter, down),
			"help":   press(sized(t, snapshotFake(), w, h), ch('?')),
			"delete": press(sized(t, snapshotFake(), w, h), ch('d')),
			"error":  sized(t, snapshotFake(), w, h).fail(errors.New("no clipboard"), "use creds get db/prod --show instead"),
			"form":   press(step(start(t, snapshotFake(), DefaultHooks()), tea.WindowSizeMsg{Width: w, Height: h}), ch('a'), enter),
		}
		for name, m := range screens {
			v := view(m)
			fitsScreen(t, m.View().Content, w, h)
			golden(t, fmt.Sprintf("%s_%dx%d", name, w, h), v)
			noSecrets(t, m)
		}
	}
}

func TestViewTooSmall(t *testing.T) {
	m := sized(t, newFake(), 20, 5)
	if v := view(m); !strings.Contains(v, "window too") || strings.Count(v, "\n") != 0 {
		t.Fatalf("view %q", v)
	}
}

func TestViewHighlightsMatches(t *testing.T) {
	m := press(start(t, newFake(), Hooks{}), append([]tea.KeyPressMsg{ch('/')}, text("wm")...)...)
	raw := m.View().Content
	amber := "38;2;255;193;7"
	if !strings.Contains(raw, amber+"m"+"w") && !strings.Contains(raw, amber+";48;2;51;51;51m"+"w") {
		t.Fatalf("match not highlighted %q", raw)
	}
}

func TestNoColor(t *testing.T) {
	var buf bytes.Buffer
	if hasColor(&buf, []string{"NO_COLOR=1", "TTY_FORCE=1", "TERM=xterm-256color", "COLORTERM=truecolor"}) {
		t.Fatal("NO_COLOR ignored")
	}
	if !hasColor(&buf, []string{"TTY_FORCE=1", "TERM=xterm-256color", "COLORTERM=truecolor"}) {
		t.Fatal("forced color not detected")
	}
	m := press(start(t, newFake(), Hooks{}), down, enter)
	m.opts.Color = true
	out := downsample(t, m.View().Content, colorprofile.ASCII)
	if regexp.MustCompile(`\x1b\[[0-9;]*[34]8;`).MatchString(out) {
		t.Fatalf("color left in %q", out)
	}
	if ansi.Strip(out) != view(m) {
		t.Fatal("text changed by downsampling")
	}
}

func TestColor256(t *testing.T) {
	out := downsample(t, press(start(t, newFake(), Hooks{}), down).View().Content, colorprofile.ANSI256)
	if strings.Contains(out, "38;2;") || !strings.Contains(out, "38;5;") {
		t.Fatalf("not downsampled %q", out)
	}
}

func downsample(t *testing.T, s string, p colorprofile.Profile) string {
	t.Helper()
	var buf bytes.Buffer
	w := colorprofile.Writer{Forward: &buf, Profile: p}
	if _, err := w.WriteString(s); err != nil {
		t.Fatal(err)
	}
	return buf.String()
}

func TestColorsOnlyInTheme(t *testing.T) {
	src := os.DirFS(".")
	files, err := fs.Glob(src, "*.go")
	if err != nil {
		t.Fatal(err)
	}
	literal := regexp.MustCompile(`lipgloss\.(Color|Red|Green|Blue|Yellow|Black|White)|"#[0-9A-Fa-f]{6}"`)
	for _, name := range files {
		if name == "theme.go" || strings.HasSuffix(name, "_test.go") {
			continue
		}
		data, err := fs.ReadFile(src, name)
		if err != nil {
			t.Fatal(err)
		}
		if literal.Match(data) {
			t.Fatalf("color literal in %s", name)
		}
	}
}

const canvasSGR = "48;2;18;18;18"

func noTerminalRecolour(t *testing.T, v tea.View) {
	t.Helper()
	if v.BackgroundColor != nil || v.ForegroundColor != nil {
		t.Fatal("View sets a terminal colour, it leaks after creds quits")
	}
	for _, osc := range []string{"\x1b]10;", "\x1b]11;", "\x1b]110", "\x1b]111"} {
		if strings.Contains(v.Content, osc) {
			t.Fatalf("content carries %q", osc)
		}
	}
}

func TestViewNeverRecoloursTheTerminal(t *testing.T) {
	m := press(start(t, newFake(), Hooks{}), down, enter)
	noTerminalRecolour(t, m.View())
	m.opts.Color = true
	noTerminalRecolour(t, m.View())
}

func TestSetupViewNeverRecoloursTheTerminal(t *testing.T) {
	m := newModel(t)
	noTerminalRecolour(t, m.View())
	m.opts.Color = true
	noTerminalRecolour(t, m.View())
}

func canvasCovers(t *testing.T, content string, width int) {
	t.Helper()
	for i, l := range strings.Split(content, "\n") {
		if n := ansi.StringWidth(l); n != width {
			t.Fatalf("line %d is %d wide, want %d: %q", i, n, width, l)
		}
		if !strings.HasPrefix(l, "\x1b["+canvasSGR+"m") {
			t.Fatalf("line %d does not open on the canvas: %q", i, l)
		}
		body := strings.TrimSuffix(l, "\x1b[m")
		if n := strings.Count(body, "\x1b[m"); n != strings.Count(body, "\x1b[m\x1b["+canvasSGR+"m") {
			t.Fatalf("line %d resets without restoring the canvas: %q", i, l)
		}
	}
}

func TestFramePaintsEveryCell(t *testing.T) {
	for _, size := range [][2]int{{80, 24}, {120, 40}} {
		w, h := size[0], size[1]
		m := sized(t, snapshotFake(), w, h)
		m.opts.Color = true
		canvasCovers(t, m.View().Content, w)
	}
}

func TestSetupFramePaintsEveryCell(t *testing.T) {
	m := newModel(t)
	m.opts.Color = true
	canvasCovers(t, m.View().Content, 80)
}

func TestFrameStaysRaggedWithoutColor(t *testing.T) {
	m := sized(t, snapshotFake(), 80, 24)
	if strings.Contains(m.View().Content, canvasSGR) {
		t.Fatal("canvas painted with colour off")
	}
}
