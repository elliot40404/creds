package cli

import (
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/elliot40404/creds/internal/app"
	"github.com/elliot40404/creds/internal/fail"
	"github.com/elliot40404/creds/internal/render"
	"github.com/spf13/cobra"
)

func TestEveryCommandHasInteractivePlan(t *testing.T) {
	t.Parallel()
	all := plans()
	token := regexp.MustCompile(`[<\[][^>\]]+[>\]]`)
	flagName := regexp.MustCompile(`(?m)^\s+(?:-\w, )?--([\w-]+)`)
	var walk func(c *cobra.Command)
	walk = func(c *cobra.Command) {
		key := planKey(c)
		pl, ok := all[key]
		if !ok {
			t.Errorf("%q has no interactive plan", key)
		}
		if c.Flag(flagInteractive) == nil {
			t.Errorf("%q lacks -i", key)
		}
		covered := append(slices.Clone(pl.asks), pl.skip...)
		for _, m := range flagName.FindAllStringSubmatch(c.LocalNonPersistentFlags().FlagUsages(), -1) {
			if !slices.Contains(covered, m[1]) {
				t.Errorf("%q flag --%s is neither asked nor skipped", key, m[1])
			}
		}
		for _, arg := range token.FindAllString(c.Use, -1) {
			if !slices.Contains(pl.asks, arg) {
				t.Errorf("%q arg %s is not asked", key, arg)
			}
		}
		for _, name := range covered {
			if !token.MatchString(name) && c.Flag(name) == nil {
				t.Errorf("%q lists unknown flag %s", key, name)
			}
		}
		for _, sub := range c.Commands() {
			walk(sub)
		}
	}
	env, _, _ := newHarness(t).env(&fake{})
	walk(NewRoot(env))
}

func TestInteractiveNeedsTerminal(t *testing.T) {
	t.Parallel()
	h := seeded(t)
	for _, args := range [][]string{{"-i"}, {"add", "-i"}, {"lock", "--interactive"}} {
		r := h.fail(&fake{noTTY: true}, args...)
		if r.code != int(fail.Usage) || !strings.Contains(r.err, "drop -i and pass flags") {
			t.Fatalf("%v: %d %q", args, r.code, r.err)
		}
	}
}

func TestInteractiveAddRerunsSame(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.init()
	f := &fake{
		inputs:    []string{"web/new", "bob", "", "pin", ""},
		picks:     []string{"login"},
		passwords: []string{loginHidden, extraHidden},
		confirms:  []bool{true},
	}
	r := h.ok(f, "add", "-i")
	noLeak(t, r, loginHidden, extraHidden)
	argv := printedCommand(t, r.err)
	want := "add web/new --secret-field password --secret-field pin --type login --username bob"
	if strings.Join(argv, " ") != want {
		t.Fatalf("command %q", argv)
	}
	h2 := newHarness(t)
	h2.init()
	h2.withStdin(t, loginHidden+"\n"+extraHidden+"\n", argv...)
	show := []string{"get", "--show", "web/new"}
	if a, b := withoutStamps(h.ok(&fake{}, show...).out), withoutStamps(h2.ok(&fake{}, show...).out); a != b || !strings.Contains(a, extraHidden) {
		t.Fatalf("rerun differs:\n%s\n%s", a, b)
	}
}

func TestInteractiveAddKeepsTypedFlags(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.init()
	f := &fake{inputs: []string{""}, passwords: []string{dbConn}}
	r := h.ok(f, "add", "db/x", "--type", "database", "--engine", "postgres", "--username", "zed", "-i")
	got := strings.Join(printedCommand(t, r.err), " ")
	if got != "add db/x --conn - --engine postgres --type database --username zed" {
		t.Fatalf("command %q", got)
	}
	if out := h.ok(&fake{}, "get", "db/x", "--field", "username").out; out != "zed\n" {
		t.Fatalf("username %q", out)
	}
}

func TestInteractiveEditAndList(t *testing.T) {
	t.Parallel()
	h := seeded(t)
	f := &fake{inputs: []string{"mail", "web/moved", "carol", "", ""}, picks: []string{"web/mail"}, passwords: []string{""}}
	r := h.ok(f, "edit", "-i")
	if got := strings.Join(printedCommand(t, r.err), " "); got != "edit web/mail --rename web/moved --username carol" {
		t.Fatalf("command %q", got)
	}
	f = &fake{inputs: []string{"type:login"}, picks: []string{"web/moved", "get", actionField, "username"}}
	r = h.ok(f, "list", "-i")
	if r.out != "carol\n" || strings.Join(printedCommand(t, r.err), " ") != "get web/moved --field username" {
		t.Fatalf("out %q err %q", r.out, r.err)
	}
	f = &fake{picks: []string{"db/prod", "get", actionFormats}}
	r = h.ok(f, "search", "prod", "-i")
	if !strings.Contains(r.out, "url") {
		t.Fatalf("formats %q", r.out)
	}
}

func TestInteractiveActionPicker(t *testing.T) {
	t.Parallel()
	h := seeded(t)
	r := h.ok(&fake{picks: []string{"lock"}}, "-i")
	if !strings.HasPrefix(r.err, "locked\ncommand: creds lock\n") {
		t.Fatalf("err %q", r.err)
	}
	r = h.run(&fake{picks: []string{"remote", "remove"}}, "-i")
	if !strings.Contains(r.err, "command: creds remote remove") {
		t.Fatalf("err %q", r.err)
	}
}

func TestInteractiveExportAndEnv(t *testing.T) {
	t.Parallel()
	h := seeded(t)
	out := filepath.Join(t.TempDir(), "x.csv")
	r := h.ok(&fake{picks: []string{"plain", app.FormatCSV}, inputs: []string{out, app.ExportPhrase}}, "export", "-i")
	if got := printedCommand(t, r.err); !slices.Equal(got, []string{"export", "--format", "csv", "--output", out, "--plain"}) {
		t.Fatalf("command %q", got)
	}
	h.ok(&fake{}, "add", "env/app", "--type", "env", "--field", "APP_MODE=dev")
	r = h.ok(&fake{inputs: []string{"type:env", ""}, picks: []string{"env/app"}}, "env", "-i")
	if r.out != "APP_MODE=dev\n" || !strings.Contains(r.err, "command: creds env env/app") {
		t.Fatalf("out %q err %q", r.out, r.err)
	}
}

func TestInteractiveStatusWithoutRemote(t *testing.T) {
	t.Parallel()
	h := seeded(t)
	r := h.ok(&fake{confirms: []bool{false}}, "status", "-i")
	if !strings.Contains(r.out, "remote: none") {
		t.Fatalf("out %q", r.out)
	}
	h.fail(&fake{}, "resolve", "-i")
}

func TestShellQuote(t *testing.T) {
	t.Parallel()
	argv := []string{"creds", "add", "a b", "it's", "", "k=v", "a,b", "@x"}
	if got := quoteLine(render.Bash, argv); got != `creds add 'a b' 'it'\''s' '' k=v a,b @x` {
		t.Fatalf("bash %s", got)
	}
	if got := quoteLine(render.Pwsh, argv); got != `creds add 'a b' 'it''s' '' k=v 'a,b' '@x'` {
		t.Fatalf("pwsh %s", got)
	}
	evil := "x';Write-Output INJECTED;'’"
	if got := quoteLine(render.Pwsh, []string{"get", evil}); got != "get 'x'';Write-Output INJECTED;''’’'" {
		t.Fatalf("pwsh evil %s", got)
	}
}

func printedCommand(t *testing.T, stderr string) []string {
	t.Helper()
	_, line, ok := strings.Cut(stderr, "command: creds ")
	if !ok {
		t.Fatalf("no command in %q", stderr)
	}
	line, _, _ = strings.Cut(line, "\n")
	line, _, _ = strings.Cut(line, "   (")
	return shellSplit(line)
}

func shellSplit(s string) []string {
	var out []string
	var cur strings.Builder
	quoted, has := false, false
	for _, r := range s {
		switch {
		case r == '\'':
			quoted, has = !quoted, true
		case r == ' ' && !quoted:
			if has {
				out = append(out, cur.String())
			}
			cur.Reset()
			has = false
		default:
			cur.WriteRune(r)
			has = true
		}
	}
	if has {
		out = append(out, cur.String())
	}
	return out
}

func (h *harness) withStdin(t *testing.T, stdin string, args ...string) {
	t.Helper()
	env, _, errb := h.env(&fake{})
	env.In = strings.NewReader(stdin)
	if code := Run(env, args); code != 0 {
		t.Fatalf("%v: %d %q", args, code, errb.String())
	}
}

func (h *harness) failStdin(t *testing.T, stdin string, args ...string) result {
	t.Helper()
	env, outb, errb := h.env(&fake{})
	env.In = strings.NewReader(stdin)
	code := Run(env, args)
	if code == 0 {
		t.Fatalf("%v: want failure, got %q", args, errb.String())
	}
	return result{code: code, out: outb.String(), err: errb.String()}
}

func TestInteractiveCopyAndImportEnv(t *testing.T) {
	t.Parallel()
	h := seeded(t)
	r := h.ok(&fake{inputs: []string{""}, picks: []string{"web/mail", app.FieldUsername}}, "copy", "-i")
	if !strings.Contains(r.err, "Copied web/mail.username") || !strings.Contains(r.err, "command: creds copy web/mail --field username") {
		t.Fatalf("err %q", r.err)
	}
	file := writeFile(t, t.TempDir(), "app.env", "MODE=dev\n")
	r = h.ok(&fake{inputs: []string{file, "env/new"}}, "import-env", "-i")
	if !strings.Contains(r.err, "imported env/new") {
		t.Fatalf("err %q", r.err)
	}
}

func TestInteractiveRunKeepsCommand(t *testing.T) {
	h := imported(t)
	args := append([]string{"run", "proj/env", "-i"}, runHelper(t, "0")...)
	r := h.ok(&fake{confirms: []bool{false}}, args...)
	if r.out != envHidden+"|" || !strings.Contains(r.err, "command: creds run proj/env -- ") {
		t.Fatalf("out %q err %q", r.out, r.err)
	}
}

func TestInteractiveAddRetriesFieldName(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.init()
	f := &fake{
		inputs:    []string{"api=key", "username", "api_key", ""},
		passwords: []string{dbConn, extraHidden},
		confirms:  []bool{true},
	}
	r := h.ok(f, "add", "db/x", "--type", "database", "--engine", "postgres", "-i")
	if len(f.warned) != 2 || !strings.Contains(f.warned[0], "field name cannot contain =") || strings.Contains(f.warned[0], "--field") {
		t.Fatalf("warned %q", f.warned)
	}
	got := strings.Join(printedCommand(t, r.err), " ")
	if got != "add db/x --conn - --engine postgres --secret-field api_key --type database" {
		t.Fatalf("command %q", got)
	}
	if out := h.ok(&fake{}, "get", "db/x", "--field", "api_key", "--show").out; out != extraHidden+"\n" {
		t.Fatalf("field %q", out)
	}
}

func TestInteractiveFieldNameGivesUp(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.init()
	f := &fake{inputs: []string{"a=1", "b=2", "c=3"}, passwords: []string{dbConn}}
	r := h.fail(f, "add", "db/x", "--type", "database", "--engine", "postgres", "-i")
	if r.code != int(fail.Usage) || !strings.Contains(r.err, "fix: type a field name without =") {
		t.Fatalf("%d %q", r.code, r.err)
	}
}

func TestAddRejectsBadPath(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.init()
	for _, p := range []string{"../etc/passwd", "/etc/passwd", "a//b", " a"} {
		r := h.fail(&fake{}, "add", "--type", "note", "--notes", "x", p)
		if r.code != int(fail.Usage) || !strings.Contains(r.err, "fix: run creds add again with a path like web/github") {
			t.Fatalf("%q: %d %q", p, r.code, r.err)
		}
	}
	f := &fake{inputs: []string{"../etc/passwd", "a/./b", "ops/x", "hi", ""}, picks: []string{"note"}}
	r := h.ok(f, "add", "-i")
	if len(f.warned) != 2 || !strings.Contains(f.warned[0], ". or .. part") {
		t.Fatalf("warned %q", f.warned)
	}
	if got := strings.Join(printedCommand(t, r.err), " "); !strings.HasPrefix(got, "add ops/x ") {
		t.Fatalf("command %q", got)
	}
	g := &fake{inputs: []string{"a/", "b/", "c/"}, picks: []string{"note"}}
	if r := h.fail(g, "add", "-i"); !strings.Contains(r.err, "has an empty part") {
		t.Fatalf("err %q", r.err)
	}
}

func TestEditRenameRejectsBadPath(t *testing.T) {
	t.Parallel()
	h := seeded(t)
	r := h.fail(&fake{}, "edit", "web/mail", "--rename", "../x")
	if r.code != int(fail.Usage) || !strings.Contains(r.err, "bad path") {
		t.Fatalf("%d %q", r.code, r.err)
	}
}

func TestInteractiveDashPathStaysPositional(t *testing.T) {
	t.Parallel()
	cmd, _, err := NewRoot(Env{}).Find([]string{"get"})
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.ParseFlags([]string{"--show"}); err != nil {
		t.Fatal(err)
	}
	iv := &interview{}
	iv.load(cmd)
	iv.args = []string{"--yes"}
	if got := iv.argv(); !slices.Equal(got, []string{"get", "--show", "--", "--yes"}) {
		t.Fatalf("argv %q", got)
	}
	iv.args = []string{"web/a"}
	if got := iv.argv(); !slices.Equal(got, []string{"get", "web/a", "--show"}) {
		t.Fatalf("argv %q", got)
	}
}

func TestInteractiveAddShowsFolders(t *testing.T) {
	t.Parallel()
	h := seeded(t)
	f := &fake{
		inputs:    []string{"web/new", "bob", "", ""},
		picks:     []string{"login"},
		passwords: []string{loginHidden},
	}
	h.ok(f, "add", "-i")
	want := "Entry path (folders: db/, web/)"
	if !slices.Contains(f.prompts, want) {
		t.Fatalf("prompts %q, want %q", f.prompts, want)
	}
}

func TestInteractiveAddWithNoFoldersHasNoHint(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.init()
	f := &fake{
		inputs:    []string{"solo", "bob", "", ""},
		picks:     []string{"login"},
		passwords: []string{loginHidden},
	}
	h.ok(f, "add", "-i")
	if !slices.Contains(f.prompts, "Entry path") {
		t.Fatalf("prompts %q", f.prompts)
	}
}

func TestInteractiveAddDetectsTheEngine(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.init()
	f := &fake{inputs: []string{""}, passwords: []string{"mongodb://u:p@h/app?retryWrites=true"}}
	r := h.ok(f, "add", "db/m", "--type", "database", "-i")
	if slices.Contains(f.prompts, "Engine") {
		t.Fatalf("still asked for the engine: %q", f.prompts)
	}
	if !slices.Contains(f.shown, "mongo (from connection string)") {
		t.Fatalf("shown %q", f.shown)
	}
	got := strings.Join(printedCommand(t, r.err), " ")
	if got != "add db/m --conn - --engine mongo --type database" {
		t.Fatalf("command %q", got)
	}
	out := h.ok(&fake{}, "get", "db/m", "--field", "host").out
	if strings.TrimSpace(out) != "h" {
		t.Fatalf("host %q", out)
	}
}

func TestInteractiveAddAsksWhenTheEngineIsUnclear(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.init()
	f := &fake{inputs: []string{""}, passwords: []string{"sslmode=require"}, selects: []int{0}}
	h.ok(f, "add", "db/d", "--type", "database", "-i")
	if !slices.Contains(f.prompts, "Connection string") || !slices.Contains(f.prompts, "Engine") {
		t.Fatalf("prompts %q", f.prompts)
	}
	if len(f.shown) != 0 {
		t.Fatalf("shown %q", f.shown)
	}
}

func TestInteractiveFiltersCombine(t *testing.T) {
	t.Parallel()
	h := seeded(t)
	f := &fake{inputs: []string{"type:database engine:postgres"}, picks: []string{"db/prod", "get", actionField, "host"}}
	h.ok(f, "list", "-i")
	if len(f.picks) != 0 {
		t.Fatalf("picks left %v", f.picks)
	}
	f = &fake{inputs: []string{"type:database engine:mongo"}}
	r := h.run(f, "list", "-i")
	if r.code == 0 {
		t.Fatalf("want no matches, got %q", r.out)
	}
}
