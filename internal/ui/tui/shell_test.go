package tui

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/elliot40404/creds/internal/app"
	"github.com/elliot40404/creds/internal/gitsync"
	"github.com/elliot40404/creds/internal/render"
	"github.com/elliot40404/creds/internal/search"
	"github.com/elliot40404/creds/internal/vault"
)

var testNow = time.Date(2026, 9, 17, 12, 0, 0, 0, time.UTC)

const (
	secretA = "alpha-secret-111"
	secretB = "bravo-secret-222"
)

type copied struct {
	path, field, as string
	shell           render.Shell
}

type fakeBackend struct {
	entries   []vault.Entry
	status    app.SyncStatus
	syncRes   string
	syncs     int
	syncErr   error
	saveErr   error
	locked    int
	expired   bool
	added     []vault.Entry
	updated   []string
	copies    []copied
	warnings  []string
	shell     render.Shell
	variesErr error
}

func newFake() *fakeBackend {
	return &fakeBackend{entries: []vault.Entry{
		{Path: "db/prod", Type: vault.TypeDatabase, Host: "h", Params: map[string]string{"engine": "postgres"}, Fields: []vault.Field{{Name: "password", Value: secretB, Secret: true}}},
		{Path: "web/mail", Type: vault.TypeLogin, Username: "me", Notes: "line1\nline2", Fields: []vault.Field{
			{Name: "password", Value: secretA, Secret: true},
			{Name: "pin", Value: secretB, Secret: true},
		}},
	}}
}

func (f *fakeBackend) find(path string) int {
	return slices.IndexFunc(f.entries, func(e vault.Entry) bool { return e.Path == path })
}

func (f *fakeBackend) List() ([]search.Summary, error) {
	if f.expired {
		return nil, fmt.Errorf("list: %w", ErrNeedPassword)
	}
	return search.SummarizeAll(f.entries), nil
}

func (f *fakeBackend) Get(path string) (vault.Entry, error) {
	if f.expired {
		return vault.Entry{}, fmt.Errorf("get: %w", ErrNeedPassword)
	}
	i := f.find(path)
	if i < 0 {
		return vault.Entry{}, errors.New("not found")
	}
	return f.entries[i], nil
}

func (f *fakeBackend) Add(e vault.Entry) (vault.Entry, error) {
	if f.saveErr != nil {
		return e, f.saveErr
	}
	f.added = append(f.added, e)
	f.entries = append(f.entries, e)
	return e, nil
}

func (f *fakeBackend) Update(path string, e vault.Entry) (vault.Entry, error) {
	if f.saveErr != nil {
		return e, f.saveErr
	}
	f.updated = append(f.updated, path)
	f.entries[f.find(path)] = e
	return e, nil
}

func (f *fakeBackend) Delete(path string) error {
	f.entries = slices.Delete(f.entries, f.find(path), f.find(path)+1)
	return nil
}

func (f *fakeBackend) Formats(string) ([]string, error) {
	return []string{"url", "psql"}, nil
}

func (f *fakeBackend) ShellVaries(_, format string) (bool, error) {
	return format != "url", f.variesErr
}

func (f *fakeBackend) RenderShell() render.Shell {
	if f.shell == "" {
		return render.Pwsh
	}
	return f.shell
}

func (f *fakeBackend) Sync() (string, error) {
	f.syncs++
	return f.syncRes, f.syncErr
}

func (f *fakeBackend) SyncStatus() (app.SyncStatus, error) {
	return f.status, nil
}

func (f *fakeBackend) Warnings() []string {
	w := f.warnings
	f.warnings = nil
	return w
}

func (f *fakeBackend) Lock() error {
	f.locked++
	return nil
}

func (f *fakeBackend) copy(path, field, as string, sh render.Shell) (string, error) {
	f.copies = append(f.copies, copied{path, field, as, sh})
	return path + "." + field + as, nil
}

func start(t *testing.T, f *fakeBackend, hooks Hooks) Model {
	t.Helper()
	m := New(Options{Backend: f, Copy: f.copy, Hooks: hooks, VaultPath: "/v", Now: func() time.Time { return testNow }})
	m = step(m, tea.WindowSizeMsg{Width: 120, Height: 30})
	return run(m, m.Init())
}

func step(m Model, msg tea.Msg) Model {
	next, cmd := m.Update(msg)
	return run(next.(Model), cmd)
}

func run(m Model, cmd tea.Cmd) Model {
	if cmd == nil {
		return m
	}
	switch msg := cmd().(type) {
	case tea.BatchMsg:
		for _, c := range msg {
			m = run(m, c)
		}
	case tea.QuitMsg, nil:
	default:
		m = step(m, msg)
	}
	return m
}

func press(m Model, ks ...tea.KeyPressMsg) Model {
	for _, k := range ks {
		m = step(m, k)
	}
	return m
}

func view(m Model) string {
	return ansi.Strip(m.View().Content)
}

func noSecrets(t *testing.T, m Model) {
	t.Helper()
	if v := view(m); strings.Contains(v, secretA) || strings.Contains(v, secretB) {
		t.Fatalf("secret in view %q", v)
	}
}

func TestNavigation(t *testing.T) {
	m := start(t, newFake(), Hooks{})
	if !strings.Contains(view(m), "web/mail") {
		t.Fatalf("list %q", view(m))
	}
	m = press(m, down, enter)
	if _, ok := m.top().(detailScreen); !ok || !strings.Contains(view(m), "username  me") {
		t.Fatalf("detail %q", view(m))
	}
	noSecrets(t, m)
	m = press(m, ch('?'))
	if !strings.Contains(view(m), "reveal field") {
		t.Fatalf("help %q", view(m))
	}
	m = press(m, esc, esc)
	if _, ok := m.top().(listScreen); !ok || m.quit {
		t.Fatal("esc should close help then go back")
	}
	if m = press(m, ch('q')); !m.quit || view(m) != "" {
		t.Fatal("q should quit from list")
	}
}

func TestQuitViewKeepsAltScreen(t *testing.T) {
	m := press(start(t, newFake(), Hooks{}), ch('q'))
	if v := m.View(); !m.quit || !v.AltScreen || v.Content != "" {
		t.Fatalf("quit view %#v", v)
	}
}

func TestCtrlCQuitsAnywhere(t *testing.T) {
	m := press(start(t, newFake(), Hooks{}), enter, ctrl('c'))
	if !m.quit {
		t.Fatal("not quit")
	}
}

func TestRevealOnlyFocusedField(t *testing.T) {
	m := press(start(t, newFake(), Hooks{}), down, enter)
	m = press(m, down, ch('r'))
	v := view(m)
	if !strings.Contains(v, secretA) || strings.Contains(v, secretB) {
		t.Fatalf("view %q", v)
	}
	m = press(m, down)
	noSecrets(t, m)
	m = press(m, ch('r'))
	if v = view(m); strings.Contains(v, secretA) || !strings.Contains(v, secretB) {
		t.Fatalf("view %q", v)
	}
	m = press(m, ch('r'))
	noSecrets(t, m)
	if !strings.Contains(view(m), "notes     line1 ...") {
		t.Fatalf("notes %q", view(m))
	}
}

func TestViewsHideSecrets(t *testing.T) {
	f := newFake()
	m := start(t, f, Hooks{})
	for _, k := range []tea.KeyPressMsg{ch('?'), esc, enter, ch('?'), esc, ch('f'), down, ch('?'), esc, esc, ch('d'), esc, ch('j'), ch('d')} {
		m = press(m, k)
		noSecrets(t, m)
	}
}

func TestCopyKeys(t *testing.T) {
	f := newFake()
	m := press(start(t, f, Hooks{}), ch('y'), down, enter, down, ch('c'), ch('y'))
	if !strings.Contains(view(m), "Copied web/mail") {
		t.Fatalf("flash %q", view(m))
	}
	m = press(m, ch('f'), esc, up, enter, ch('f'), down, enter, ch('1'))
	want := []copied{
		{path: "db/prod"},
		{path: "web/mail", field: "password"},
		{path: "web/mail"},
		{path: "db/prod", as: "psql", shell: render.Pwsh},
	}
	if !slices.Equal(f.copies, want) {
		t.Fatalf("copies %v", f.copies)
	}
	if !strings.Contains(view(m), "Copied db/prod.psql") {
		t.Fatalf("flash %q", view(m))
	}
}

func TestCopyFailureShowsFix(t *testing.T) {
	m := start(t, newFake(), Hooks{})
	m.opts.Copy = func(string, string, string, render.Shell) (string, error) {
		return "", errors.New("no clipboard\nreveal it instead")
	}
	m = press(m, ch('y'))
	if v := view(m); !strings.Contains(v, "no clipboard. use creds get db/prod --show instead") {
		t.Fatalf("view %q", v)
	}
}

func failingCopy(m Model, value string) Model {
	m.opts.Copy = func(path, _, _ string, _ render.Shell) (string, error) {
		return "", &CopyFailed{Label: path + " as url", Value: value, Err: errors.New("clipboard busy")}
	}
	return press(m, ch('y'))
}

func TestCopyFailedOffersValue(t *testing.T) {
	m := failingCopy(start(t, newFake(), Hooks{}), secretA)
	if v := view(m); !strings.Contains(v, "clipboard failed: clipboard busy") || !strings.Contains(v, "show db/prod as url here") {
		t.Fatalf("view %q", v)
	}
	noSecrets(t, m)
	m = press(m, ch('y'))
	if v := view(m); !strings.Contains(v, secretA) || !strings.Contains(v, "p print after quit") {
		t.Fatalf("view %q", v)
	}
	m = press(m, ch('p'))
	if m.printOnQuit != secretA || !m.quit {
		t.Fatalf("print %q quit %v", m.printOnQuit, m.quit)
	}
}

func TestCopyFailedDeclineHidesValue(t *testing.T) {
	for _, k := range []tea.KeyPressMsg{ch('n'), esc} {
		m := press(failingCopy(start(t, newFake(), Hooks{}), secretA), k)
		if len(m.stack) != 1 || m.printOnQuit != "" {
			t.Fatalf("stack %d print %q", len(m.stack), m.printOnQuit)
		}
		noSecrets(t, m)
	}
	m := press(failingCopy(start(t, newFake(), Hooks{}), secretA), ch('y'), esc)
	if len(m.stack) != 1 || m.quit {
		t.Fatalf("stack %d quit %v", len(m.stack), m.quit)
	}
	noSecrets(t, m)
}

func TestCopyFailedValueEscaped(t *testing.T) {
	m := press(failingCopy(start(t, newFake(), Hooks{}), "a\x1b]52;c;x\x07b"), ch('y'))
	if v := m.View().Content; strings.Contains(v, "\x1b]52") || strings.Contains(v, "\x07") {
		t.Fatalf("control chars in view %q", v)
	}
}

func TestDeleteConfirm(t *testing.T) {
	f := newFake()
	m := press(start(t, f, Hooks{}), ch('d'))
	if v := view(m); !strings.Contains(v, "Delete this entry?") || !strings.Contains(v, "entry db/prod") {
		t.Fatalf("view %q", v)
	}
	m = press(m, ch('n'))
	if len(f.entries) != 2 || len(m.stack) != 1 {
		t.Fatalf("entries %d stack %d", len(f.entries), len(m.stack))
	}
	m = press(m, down, enter, ch('d'), ch('y'))
	if len(f.entries) != 1 || len(m.stack) != 1 || !strings.Contains(view(m), "Deleted web/mail") {
		t.Fatalf("entries %d view %q", len(f.entries), view(m))
	}
	if strings.Contains(view(m), "web/mail  login") {
		t.Fatal("list not reloaded")
	}
}

func TestDeleteConfirmNamesLongPath(t *testing.T) {
	long := "team/" + strings.Repeat("x", 60) + "/db"
	f := newFake()
	f.entries = []vault.Entry{{Path: long, Type: vault.TypeNote}}
	m := step(press(start(t, f, Hooks{}), ch('d')), tea.WindowSizeMsg{Width: 80, Height: 24})
	if v := view(m); !strings.Contains(v, "entry "+long) {
		t.Fatalf("view %q", v)
	}
}

func TestSyncResults(t *testing.T) {
	f := newFake()
	f.syncRes = "pushed 1 commit"
	m := press(start(t, f, Hooks{}), ch('s'))
	if !strings.Contains(view(m), "Sync: pushed 1 commit") || !m.op.idle() {
		t.Fatalf("view %q", view(m))
	}
	f.syncErr = &app.ConflictError{Paths: []string{"db/prod", "web/mail"}}
	m = press(m, ch('s'))
	if !strings.Contains(view(m), "sync conflicts in db/prod, web/mail. fix with creds resolve") {
		t.Fatalf("view %q", view(m))
	}
	f.syncErr = errors.New("remote hung up\ndetails")
	m = press(m, enter, ch('s'))
	if v := view(m); !strings.Contains(v, "sync failed: remote hung up. check the remote") || strings.Contains(v, "details") {
		t.Fatalf("view %q", v)
	}
	f.syncErr = fmt.Errorf("sync: %w", gitsync.ErrNoRemote)
	m = press(m, enter, ch('s'))
	if v := view(m); !strings.Contains(v, "sync failed: no remote configured. run creds remote add <url>") || strings.Contains(v, "gitsync") {
		t.Fatalf("view %q", v)
	}
}

func TestBusyRejectsActions(t *testing.T) {
	m := start(t, newFake(), Hooks{})
	for _, msg := range []tea.Msg{syncMsg{}, openMsg{path: "web/mail"}} {
		next, _ := m.Update(msg)
		m = next.(Model)
	}
	if m.op.kind != opSync || !strings.Contains(view(m), "still working") {
		t.Fatalf("view %q", view(m))
	}
}

func TestStatusLine(t *testing.T) {
	f := newFake()
	f.status = app.SyncStatus{
		Remote:    "https://bob:" + "tok3n" + "@example.com/v.git",
		LastSync:  time.Date(2026, 9, 17, 11, 55, 0, 0, time.UTC),
		LastError: "network down",
		Conflicts: []string{"a", "b"},
	}
	v := view(start(t, f, Hooks{}))
	for _, want := range []string{"vault /v", "remote https://example.com/v.git", "synced 5m ago", "conflicts 2", "last sync failed: network down. press s to retry"} {
		if !strings.Contains(v, want) {
			t.Fatalf("missing %q in %q", want, v)
		}
	}
	if strings.Contains(v, "tok3n") {
		t.Fatal("remote credentials shown")
	}
	f.status = app.SyncStatus{}
	if v = view(start(t, f, Hooks{})); !strings.Contains(v, "remote none") || strings.Contains(v, "synced") {
		t.Fatalf("view %q", v)
	}
}

func TestStatusLineMapsGitError(t *testing.T) {
	f := newFake()
	f.status = app.SyncStatus{
		Remote:    "https://h/v.git",
		LastError: "git fetch -q origin: fatal: unable to access 'https://h/v.git/': Could not resolve host: h",
	}
	v := view(start(t, f, Hooks{}))
	if !strings.Contains(v, "last sync failed: cannot reach the git host. check the network") || strings.Contains(v, "git fetch") {
		t.Fatalf("view %q", v)
	}
}

func TestWarningsFlash(t *testing.T) {
	f := newFake()
	f.warnings = []string{"vault is readable by others", "session is readable by others"}
	v := view(start(t, f, Hooks{}))
	if !strings.Contains(v, "warning: vault is readable by others; session is readable by others") {
		t.Fatalf("view %q", v)
	}
	if f.warnings != nil {
		t.Fatal("warnings not drained")
	}
}

func TestLockQuits(t *testing.T) {
	f := newFake()
	m := press(start(t, f, Hooks{}), enter, ch('L'))
	if f.locked != 1 || !m.quit {
		t.Fatalf("locked %d", f.locked)
	}
}

func TestFormHooksMissing(t *testing.T) {
	m := press(start(t, newFake(), Hooks{}), ch('a'))
	if !strings.Contains(view(m), "not ready yet. use creds add") {
		t.Fatalf("view %q", view(m))
	}
	m = press(m, enter, ch('e'))
	if !strings.Contains(view(m), "use creds edit db/prod") {
		t.Fatalf("view %q", view(m))
	}
}

type fakeForm struct {
	orig vault.Entry
	err  error
}

func (f fakeForm) Update(msg tea.Msg) (Screen, tea.Cmd) {
	switch msg := msg.(type) {
	case FormError:
		f.err = msg.Err
	case tea.KeyPressMsg:
		switch msg.String() {
		case "esc":
			return f, send(FormResult{Canceled: true})
		case "enter":
			e := f.orig
			if e.Path == "" {
				e = vault.Entry{Path: "new/one", Type: vault.TypeNote}
			}
			e.Notes = "changed"
			return f, send(FormResult{Entry: e})
		}
	}
	return f, nil
}

func (f fakeForm) View(int, int) string {
	if f.err != nil {
		return "form error: " + f.err.Error()
	}
	return "form " + f.orig.Path
}

func formHooks() Hooks {
	return Hooks{
		Add:  func([]string) Screen { return fakeForm{} },
		Edit: func(orig vault.Entry) Screen { return fakeForm{orig: orig} },
	}
}

func TestAddForm(t *testing.T) {
	f := newFake()
	m := press(start(t, f, formHooks()), ch('a'), ch('?'), esc)
	if len(m.stack) != 1 || m.help {
		t.Fatalf("stack %d help %v", len(m.stack), m.help)
	}
	f.saveErr = errors.New("path already exists")
	m = press(m, ch('a'), enter)
	if !strings.Contains(view(m), "form error: path already exists") || len(m.stack) != 2 {
		t.Fatalf("view %q", view(m))
	}
	f.saveErr = nil
	m = press(m, enter)
	if len(f.added) != 1 || len(m.stack) != 1 || !strings.Contains(view(m), "new/one   note") {
		t.Fatalf("added %d view %q", len(f.added), view(m))
	}
}

func TestEditForm(t *testing.T) {
	f := newFake()
	m := press(start(t, f, formHooks()), enter, ch('e'))
	if !strings.Contains(view(m), "form db/prod") {
		t.Fatalf("view %q", view(m))
	}
	m = press(m, enter)
	d, ok := m.top().(detailScreen)
	if !ok || d.entry.Notes != "changed" || !slices.Equal(f.updated, []string{"db/prod"}) {
		t.Fatalf("updated %v", f.updated)
	}
	if !strings.Contains(view(m), "Saved db/prod") {
		t.Fatalf("view %q", view(m))
	}
	noSecrets(t, m)
}
