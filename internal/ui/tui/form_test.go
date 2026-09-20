package tui

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/elliot40404/creds/internal/app"
	"github.com/elliot40404/creds/internal/vault"
)

func formView(s Screen) string {
	return ansi.Strip(s.View(120, 30))
}

func pickType(t *testing.T, ty vault.Type) Screen {
	t.Helper()
	i := slices.Index(app.Types(), ty)
	s := NewAddForm(nil)
	for range i {
		s, _ = keys(s, down)
	}
	s, _ = keys(s, enter)
	if !strings.Contains(formView(s), "Add "+string(ty)) {
		t.Fatalf("view %q", formView(s))
	}
	return s
}

func saved(t *testing.T, msg tea.Msg) vault.Entry {
	t.Helper()
	res, ok := msg.(FormResult)
	if !ok || res.Canceled {
		t.Fatalf("msg %#v", msg)
	}
	return res.Entry
}

func fld(name, value string, secret bool) vault.Field {
	return vault.Field{Name: name, Value: value, Secret: secret}
}

func TestAddEachType(t *testing.T) {
	cases := []struct {
		ty    vault.Type
		steps []any
		want  vault.Entry
	}{
		{
			vault.TypeLogin,
			[]any{"web/a", tabKey, "me", tabKey, secretA, tabKey, "https://a"},
			vault.Entry{Username: "me", URL: "https://a", Fields: []vault.Field{fld("password", secretA, true)}},
		},
		{
			vault.TypeAPI,
			[]any{"api/a", tabKey, "https://api", tabKey, secretA},
			vault.Entry{URL: "https://api", Fields: []vault.Field{fld("key", secretA, true)}},
		},
		{
			vault.TypeSSH,
			[]any{"ssh/a", tabKey, "host1", tabKey, "root", tabKey, "22", tabKey, secretA},
			vault.Entry{Host: "host1", Username: "root", Fields: []vault.Field{fld("port", "22", false), fld("password", secretA, true)}},
		},
		{vault.TypeNote, []any{"note/a", tabKey, "hello"}, vault.Entry{Notes: "hello"}},
		{vault.TypeCommand, []any{"cmd/a", tabKey, "make run"}, vault.Entry{Fields: []vault.Field{fld("command", "make run", false)}}},
		{vault.TypeEnv, []any{"env/a"}, vault.Entry{}},
		{vault.TypeGeneric, []any{"gen/a"}, vault.Entry{}},
		{
			vault.TypeDatabase,
			[]any{"db/a", tabKey, rightKey, tabKey, "redis://:" + secretA + "@cache:6380/2"},
			vault.Entry{Host: "cache", Fields: []vault.Field{
				fld("engine", "redis", false), fld("scheme", "redis", false), fld("port", "6380", false),
				fld("database", "2", false), fld("password", secretA, true),
			}},
		},
	}
	for _, tc := range cases {
		t.Run(string(tc.ty), func(t *testing.T) {
			s := pickType(t, tc.ty)
			s, _ = formSteps(s, tc.steps...)
			if v := formView(s); strings.Contains(v, secretA) {
				t.Fatalf("secret in view %q", v)
			}
			_, msg := formSteps(s, ctrl('s'))
			got := saved(t, msg)
			tc.want.Type, tc.want.Path = tc.ty, tc.steps[0].(string)
			if got.Path != tc.want.Path || got.Type != tc.want.Type || got.Username != tc.want.Username ||
				got.Host != tc.want.Host || got.URL != tc.want.URL || got.Notes != tc.want.Notes ||
				!slices.Equal(got.Fields, tc.want.Fields) {
				t.Fatalf("got  %+v\nwant %+v", got, tc.want)
			}
		})
	}
}

func TestAddCustomSecretField(t *testing.T) {
	s := pickType(t, vault.TypeGeneric)
	s, _ = formSteps(s, "gen/x", ctrl('n'), "pin", enter, secretB)
	if !strings.Contains(formView(s), secretB) {
		t.Fatalf("plain custom field should show %q", formView(s))
	}
	s, _ = formSteps(s, ctrl('t'))
	v := formView(s)
	if strings.Contains(v, secretB) || !strings.Contains(v, "pin (secret)") {
		t.Fatalf("view %q", v)
	}
	s, msg := formSteps(s, ctrl('r'))
	if strings.Contains(formView(s), secretB) {
		t.Fatal("reveal showed the field before the session check")
	}
	s, _ = s.Update(unmaskedMsg{unmaskMsg: msg.(unmaskMsg)})
	if !strings.Contains(formView(s), secretB) {
		t.Fatal("reveal did not show focused field")
	}
	s, _ = formSteps(s, ctrl('r'))
	if strings.Contains(formView(s), secretB) {
		t.Fatal("hide did not mask")
	}
	s, msg = formSteps(s, enter)
	got := saved(t, msg)
	if !slices.Equal(got.Fields, []vault.Field{fld("pin", secretB, true)}) {
		t.Fatalf("fields %+v", got.Fields)
	}
	if !strings.Contains(formView(s), "saving") {
		t.Fatalf("view %q", formView(s))
	}
}

func TestCustomFieldNameErrors(t *testing.T) {
	s := pickType(t, vault.TypeLogin)
	for _, name := range []string{"", "username", "password", "path"} {
		n, _ := formSteps(s, ctrl('n'), name, enter)
		if f := n.(formScreen); f.naming == nil || f.nameErr == "" || !strings.Contains(formView(n), f.nameErr) {
			t.Fatalf("name %q accepted: %q", name, formView(n))
		}
	}
	n, _ := formSteps(s, ctrl('n'), "x", esc)
	if n.(formScreen).naming != nil || len(n.(formScreen).rows) != len(s.(formScreen).rows) {
		t.Fatal("esc should close name prompt without adding")
	}
}

func TestEditKeepsSecret(t *testing.T) {
	orig := vault.Entry{Path: "web/mail", Type: vault.TypeLogin, Username: "me", Fields: []vault.Field{
		fld("password", secretA, true), fld("pin", secretB, true), fld("note", "n", false),
	}}
	s := NewEditForm(orig.Clone())
	v := formView(s)
	if strings.Contains(v, secretA) || strings.Contains(v, secretB) || !strings.Contains(v, "blank keeps current") || !strings.Contains(v, "me") {
		t.Fatalf("view %q", v)
	}
	_, msg := formSteps(s, tabKey, back, back, "you", ctrl('s'))
	got := saved(t, msg)
	if got.Username != "you" || got.ID != orig.ID || !slices.Equal(got.Fields, orig.Fields) {
		t.Fatalf("got %+v", got)
	}
	_, msg = formSteps(s, tabKey, tabKey, "new-pass", tabKey, tabKey, tabKey, ctrl('t'), ctrl('s'))
	got = saved(t, msg)
	want := []vault.Field{fld("password", "new-pass", true), fld("pin", secretB, true), fld("note", "n", true)}
	if !slices.Equal(got.Fields, want) {
		t.Fatalf("fields %+v", got.Fields)
	}
	if orig.Fields[0].Value != secretA {
		t.Fatal("original entry changed")
	}
}

func TestEditDatabaseRows(t *testing.T) {
	e := vault.Entry{Path: "db/a", Type: vault.TypeDatabase, Host: "h", Fields: []vault.Field{
		fld("engine", "postgres", false), fld("password", secretA, true),
	}}
	s := NewEditForm(e)
	v := formView(s)
	if strings.Contains(v, secretA) || !strings.Contains(v, "engine") || strings.Contains(v, rowConn) {
		t.Fatalf("view %q", v)
	}
	_, msg := formSteps(s, ctrl('s'))
	if got := saved(t, msg); !slices.Equal(got.Fields, e.Fields) || got.Host != "h" {
		t.Fatalf("got %+v", got)
	}
}

func TestEditRemoveFieldsAndParams(t *testing.T) {
	e := vault.Entry{
		Path: "db/a", Type: vault.TypeDatabase, Host: "h",
		Params: map[string]string{"app": "x", "sslmode": "require"},
		Fields: []vault.Field{fld("engine", "postgres", false), fld("password", secretA, true), fld("pin", secretB, true)},
	}
	s := NewEditForm(e.Clone())
	if v := formView(s); !strings.Contains(v, "sslmode (param)") || !strings.Contains(v, "require") {
		t.Fatalf("view %q", v)
	}
	s, _ = formSteps(s, up, ctrl('u'), "verify-full", up, ctrl('d'), up, ctrl('d'))
	_, msg := formSteps(s, ctrl('s'))
	got := saved(t, msg)
	want := []vault.Field{fld("engine", "postgres", false), fld("password", secretA, true)}
	if !slices.Equal(got.Fields, want) || len(got.Params) != 1 || got.Params["sslmode"] != "verify-full" {
		t.Fatalf("got %+v", got)
	}
	if len(e.Params) != 2 || len(e.Fields) != 3 {
		t.Fatal("original entry changed")
	}
	_, msg = formSteps(s, ctrl('n'), "pin", enter, "new", ctrl('s'))
	if got := saved(t, msg); !slices.Contains(got.Fields, fld("pin", "new", false)) || len(got.Fields) != 3 {
		t.Fatalf("readd %+v", got.Fields)
	}
	_, msg = formSteps(s, ctrl('u'), ctrl('s'))
	if got := saved(t, msg); got.Params != nil {
		t.Fatalf("blank param kept %+v", got.Params)
	}
}

func TestRemoveOnlyCustomRows(t *testing.T) {
	s := NewEditForm(vault.Entry{Path: "web/a", Type: vault.TypeLogin, Username: "me"})
	for _, n := range []int{0, 1} {
		r, _ := formSteps(s, formStepsN(tabKey, n)...)
		r, _ = formSteps(r, ctrl('d'))
		if f := r.(formScreen); len(f.rows) != len(s.(formScreen).rows) || !strings.Contains(formView(r), "cannot be removed") {
			t.Fatalf("row %d removed: %q", n, formView(r))
		}
	}
	a, _ := formSteps(pickType(t, vault.TypeNote), "n/a", ctrl('n'), "pin", enter, "1", ctrl('d'))
	_, msg := formSteps(a, ctrl('s'))
	if got := saved(t, msg); len(got.Fields) != 0 {
		t.Fatalf("fields %+v", got.Fields)
	}
}

func formStepsN(step any, n int) []any {
	out := make([]any, n)
	for i := range out {
		out[i] = step
	}
	return out
}

func TestFormValidation(t *testing.T) {
	s, msg := formSteps(pickType(t, vault.TypeLogin), tabKey, "me", ctrl('s'))
	if msg != nil || s.(formScreen).focus != 0 || !strings.Contains(formView(s), "path is empty") {
		t.Fatalf("msg %v view %q", msg, formView(s))
	}
	s, _ = formSteps(s, "web/a")
	if strings.Contains(formView(s), "path is empty") {
		t.Fatal("error should clear on typing")
	}
	db := pickType(t, vault.TypeDatabase)
	s, msg = formSteps(db, "db/a", ctrl('s'))
	if msg != nil || !strings.Contains(formView(s), "connection string is empty") {
		t.Fatalf("view %q", formView(s))
	}
	s, msg = formSteps(db, "db/a", tabKey, tabKey, "mysql://"+secretA+"@h", ctrl('s'))
	v := formView(s)
	if msg != nil || !strings.Contains(v, "invalid connection string") || strings.Contains(v, secretA) {
		t.Fatalf("view %q", v)
	}
	s, msg = formSteps(db, "db/a", tabKey, tabKey, "postgres://u:p@h:1/d", ctrl('n'), "port", enter, "x", ctrl('s'))
	if msg != nil || !strings.Contains(formView(s), "field port already exists") {
		t.Fatalf("msg %v view %q", msg, formView(s))
	}
}

func TestFormSaveErrorAndCancel(t *testing.T) {
	s, _ := formSteps(pickType(t, vault.TypeNote), "n/a", ctrl('s'))
	s, _ = s.Update(FormError{Err: errors.New("duplicate entry path: n/a\nmore")})
	f := s.(formScreen)
	if f.saving || !strings.Contains(formView(s), "press ctrl+s") || strings.Contains(formView(s), "more") {
		t.Fatalf("view %q", formView(s))
	}
	if _, msg := formSteps(s, esc); !canceled(msg) {
		t.Fatalf("msg %#v", msg)
	}
	if _, msg := keys(NewAddForm(nil), esc); !canceled(msg) {
		t.Fatalf("picker esc %#v", msg)
	}
	if NewAddForm(nil).(formScreen).typing() || !s.(formScreen).typing() {
		t.Fatal("typing state")
	}
}

func TestFormPasteAndHelp(t *testing.T) {
	s := pickType(t, vault.TypeNote)
	s, _ = s.Update(tea.PasteMsg{Content: "a/b\r\n"})
	if s.(formScreen).rows[0].input.String() != "a/b" {
		t.Fatalf("paste %q", s.(formScreen).rows[0].input.String())
	}
	if len(s.(formScreen).keys()) == 0 || len(NewAddForm(nil).(formScreen).keys()) == 0 {
		t.Fatal("no help keys")
	}
}

func TestHooksInShell(t *testing.T) {
	f := newFake()
	m := start(t, f, DefaultHooks())
	next, cmd := m.Update(addMsg{})
	m = run(next.(Model), cmd)
	m = press(m, enter)
	m = press(m, text("web/new")...)
	m = press(m, tabKey)
	m = press(m, text("bob")...)
	m = press(m, tabKey)
	m = press(m, text(secretA)...)
	noSecrets(t, m)
	m = press(m, ctrl('s'))
	if len(f.added) != 1 || f.added[0].Path != "web/new" || f.added[0].Username != "bob" {
		t.Fatalf("added %+v", f.added)
	}
	if _, ok := m.top().(formScreen); ok {
		t.Fatal("form still open after save")
	}
}

func canceled(msg tea.Msg) bool {
	res, ok := msg.(FormResult)
	return ok && res.Canceled
}

func TestEditMasksSecretParams(t *testing.T) {
	e := vault.Entry{Path: "db/a", Type: vault.TypeDatabase, Host: "h", Params: map[string]string{"password": secretA}}
	s := NewEditForm(e)
	if v := formView(s); strings.Contains(v, secretA) || !strings.Contains(v, "password (param)") {
		t.Fatalf("view %q", v)
	}
	if _, msg := formSteps(s, ctrl('s')); saved(t, msg).Params["password"] != secretA {
		t.Fatal("param changed")
	}
}

func addFormWith(t *testing.T, paths []string) Screen {
	t.Helper()
	i := slices.Index(app.Types(), vault.TypeLogin)
	s := NewAddForm(paths)
	for range i {
		s, _ = keys(s, down)
	}
	s, _ = keys(s, enter)
	return s
}

func pathRow(s Screen) formRow {
	return s.(formScreen).rows[0]
}

var samplePaths = []string{"work/db/pg", "work/db/redis", "web/mail"}

func TestFormTabCompletesAFolder(t *testing.T) {
	s := addFormWith(t, samplePaths)
	s, _ = keys(s, text("wo")...)
	if g := pathRow(s).ghost; g != "rk/" {
		t.Fatalf("ghost %q", g)
	}
	if !strings.Contains(formView(s), "wo_rk/") {
		t.Fatalf("ghost not shown after the cursor: %q", formView(s))
	}
	s, _ = keys(s, tabKey)
	if v := pathRow(s).input.String(); v != "work/" {
		t.Fatalf("value %q", v)
	}
	if pathRow(s).ghost != "db/" {
		t.Fatalf("ghost %q", pathRow(s).ghost)
	}
	s, _ = keys(s, tabKey)
	if v := pathRow(s).input.String(); v != "work/db/" {
		t.Fatalf("value %q", v)
	}
	if s.(formScreen).focus != 0 {
		t.Fatal("tab moved focus while completing")
	}
}

func TestFormTabMovesFocusWithoutAGhost(t *testing.T) {
	s := addFormWith(t, samplePaths)
	s, _ = keys(s, text("zzz")...)
	if pathRow(s).ghost != "" {
		t.Fatalf("ghost %q", pathRow(s).ghost)
	}
	s, _ = keys(s, tabKey)
	if s.(formScreen).focus != 1 {
		t.Fatalf("focus %d", s.(formScreen).focus)
	}
}

func TestFormTabWithNoPathsMovesFocus(t *testing.T) {
	s := addFormWith(t, nil)
	s, _ = keys(s, text("wo")...)
	if pathRow(s).ghost != "" {
		t.Fatalf("ghost %q", pathRow(s).ghost)
	}
	s, _ = keys(s, tabKey)
	if s.(formScreen).focus != 1 {
		t.Fatalf("focus %d", s.(formScreen).focus)
	}
}

func TestFormWarnsOnADuplicatePath(t *testing.T) {
	s := addFormWith(t, samplePaths)
	s, _ = keys(s, text("web/mail")...)
	r := pathRow(s)
	if r.err == "" || !strings.Contains(r.err, "web/mail") {
		t.Fatalf("err %q", r.err)
	}
	if r.err != saveHint(fmt.Errorf("%w: %s", vault.ErrDuplicatePath, "web/mail")) {
		t.Fatalf("wording drifted: %q", r.err)
	}
	if !strings.Contains(formView(s), r.err) {
		t.Fatal("duplicate warning not shown")
	}
	s, _ = keys(s, back)
	if pathRow(s).err != "" {
		t.Fatalf("err kept after backspace: %q", pathRow(s).err)
	}
}

func TestFormFolderPrefixIsNotADuplicate(t *testing.T) {
	s := addFormWith(t, samplePaths)
	s, _ = keys(s, text("work/")...)
	if e := pathRow(s).err; e != "" {
		t.Fatalf("folder flagged as duplicate: %q", e)
	}
}

func TestEditFormHasNoCompletion(t *testing.T) {
	s := NewEditForm(vault.Entry{Path: "web/mail", Type: vault.TypeLogin})
	s, _ = keys(s, back)
	if pathRow(s).ghost != "" || pathRow(s).err != "" {
		t.Fatalf("edit form suggests: %+v", pathRow(s))
	}
}

const connUser = "u:p@"

func mongoURL(rest string) string {
	return "mongodb://" + connUser + rest
}

func dbForm(t *testing.T) Screen {
	t.Helper()
	s := pickType(t, vault.TypeDatabase)
	s, _ = keys(s, tabKey, tabKey)
	if s.(formScreen).rows[s.(formScreen).focus].kind != kindConn {
		t.Fatal("not on the connection row")
	}
	return s
}

func engineOf(s Screen) string {
	f := s.(formScreen)
	return f.engines[f.engine]
}

func TestFormSetsTheEngineFromTheConnString(t *testing.T) {
	s := dbForm(t)
	if engineOf(s) != app.Engines()[0] {
		t.Fatalf("engine starts at %q", engineOf(s))
	}
	s, _ = s.Update(tea.PasteMsg{Content: mongoURL("h/db?retryWrites=true")})
	if engineOf(s) != "mongo" {
		t.Fatalf("engine %q", engineOf(s))
	}
	if s.(formScreen).engineNote == "" {
		t.Fatal("no note on the engine row")
	}
	if !strings.Contains(formView(s), "set from the connection string") {
		t.Fatalf("note not shown: %q", formView(s))
	}
}

func TestFormKeepsAManuallyPickedEngine(t *testing.T) {
	s := pickType(t, vault.TypeDatabase)
	s, _ = keys(s, tabKey)
	s, _ = keys(s, rightKey)
	picked := engineOf(s)
	s, _ = keys(s, tabKey)
	s, _ = s.Update(tea.PasteMsg{Content: mongoURL("h/db")})
	if engineOf(s) != picked {
		t.Fatalf("engine changed to %q, want %q", engineOf(s), picked)
	}
	if s.(formScreen).engineNote != "" {
		t.Fatalf("note %q", s.(formScreen).engineNote)
	}
}

func TestFormLeavesTheEngineWhenUndetectable(t *testing.T) {
	s := dbForm(t)
	s, _ = s.Update(tea.PasteMsg{Content: "sslmode=require"})
	if engineOf(s) != app.Engines()[0] || s.(formScreen).engineNote != "" {
		t.Fatalf("engine %q note %q", engineOf(s), s.(formScreen).engineNote)
	}
}

func TestFormSavesADetectedMongoEntry(t *testing.T) {
	s := dbForm(t)
	s, _ = keys(s, up, up)
	s, _ = keys(s, text("db/m")...)
	s, _ = keys(s, tabKey, tabKey)
	s, _ = s.Update(tea.PasteMsg{Content: mongoURL("mongo.host/app?retryWrites=true")})
	_, msg := keys(s, ctrl('s'))
	e := saved(t, msg)
	got, _ := app.Lookup(e, app.FieldEngine)
	if got != "mongo" {
		t.Fatalf("engine %q", got)
	}
	if e.Host != "mongo.host" {
		t.Fatalf("host %q, the connection string was parsed as the wrong engine", e.Host)
	}
}
