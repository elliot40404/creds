package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/charmbracelet/x/ansi"
	"github.com/elliot40404/creds/internal/app"
	"github.com/elliot40404/creds/internal/render"
)

func TestFormatsNumberCopies(t *testing.T) {
	t.Parallel()
	f := newFormats("work/db", []string{"dotenv", "env", "psql"}, render.Pwsh)
	_, msg := keys(f, ch('3'))
	got, ok := msg.(copyMsg)
	if !ok || got.path != "work/db" || got.as != "psql" {
		t.Fatalf("msg %+v", msg)
	}
}

func TestFormatsNumberPastEndDoesNothing(t *testing.T) {
	t.Parallel()
	f := newFormats("work/db", []string{"dotenv", "env"}, render.Pwsh)
	if _, msg := keys(f, ch('9')); msg != nil {
		t.Fatalf("msg %+v", msg)
	}
}

func TestFormatsViewNumbersRows(t *testing.T) {
	t.Parallel()
	f := newFormats("work/db", []string{"dotenv", "env"}, render.Pwsh)
	out := ansi.Strip(f.View(40, 10))
	if !strings.Contains(out, "1 dotenv") || !strings.Contains(out, "2 env") {
		t.Fatalf("view %q", out)
	}
}

func TestSetupSelectNumberPicks(t *testing.T) {
	t.Parallel()
	s := setupSelect{prompt: "pick", options: []string{"a", "b", "c"}}
	_, msg := keys(s, ch('2'))
	got, ok := msg.(setupAns)
	if !ok || !got.ok || got.idx != 1 || got.text != "b" {
		t.Fatalf("msg %+v", msg)
	}
}

func TestAddFormNumberPicksType(t *testing.T) {
	t.Parallel()
	ty := app.Types()[1]
	s, _ := keys(NewAddForm(nil), ch('2'))
	if !strings.Contains(formView(s), "Add "+string(ty)) {
		t.Fatalf("view %q", formView(s))
	}
}

func TestFormatsShiftNumberCopiesOtherShell(t *testing.T) {
	t.Parallel()
	f := newFormats("work/db", []string{"dotenv", "env", "psql"}, render.Pwsh)
	_, msg := keys(f, ch('#'))
	got, ok := msg.(copyMsg)
	if !ok || got.as != "psql" || got.shell != render.Bash {
		t.Fatalf("msg %+v", msg)
	}
}

func TestFormatsShiftNumberUsesBaseCode(t *testing.T) {
	t.Parallel()
	f := newFormats("work/db", []string{"dotenv", "env"}, render.Bash)
	key := tea.KeyPressMsg{Code: '"', Text: "\"", Mod: tea.ModShift, BaseCode: '2'}
	_, msg := keys(f, key)
	got, ok := msg.(copyMsg)
	if !ok || got.as != "env" || got.shell != render.Pwsh {
		t.Fatalf("msg %+v", msg)
	}
}

func TestFormatsEnterAsksForShell(t *testing.T) {
	t.Parallel()
	f := newFormats("work/db", []string{"dotenv", "psql"}, render.Pwsh)
	_, msg := keys(f, down, enter)
	if got, ok := msg.(shellPickMsg); !ok || got.as != "psql" || got.path != "work/db" {
		t.Fatalf("msg %+v", msg)
	}
}

func TestShellsScreenCopies(t *testing.T) {
	t.Parallel()
	s := newShells("work/db", "psql", render.Pwsh)
	if out := ansi.Strip(s.View(60, 10)); !strings.Contains(out, "1 pwsh  your default") || !strings.Contains(out, "2 bash") {
		t.Fatalf("view %q", out)
	}
	_, msg := keys(s, ch('2'))
	if got, ok := msg.(copyMsg); !ok || got.shell != render.Bash || got.as != "psql" {
		t.Fatalf("msg %+v", msg)
	}
	_, msg = keys(s, down, enter)
	if got, ok := msg.(copyMsg); !ok || got.shell != render.Bash {
		t.Fatalf("enter msg %+v", msg)
	}
}

func TestShellPickSkippedWhenShellDoesNotMatter(t *testing.T) {
	f := newFake()
	m := press(start(t, f, Hooks{}), enter, ch('f'), enter)
	if len(f.copies) != 1 || f.copies[0].as != "url" || f.copies[0].shell != "" {
		t.Fatalf("copies %+v", f.copies)
	}
	if !strings.Contains(view(m), "Copied db/prod.url") {
		t.Fatalf("flash %q", view(m))
	}
}
