package cli

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/elliot40404/creds/internal/app"
	"github.com/elliot40404/creds/internal/fail"
	"github.com/elliot40404/creds/internal/gitsync"
	"github.com/elliot40404/creds/internal/vault"
)

func TestExitCodes(t *testing.T) {
	t.Parallel()
	h := seeded(t)
	fresh := newHarness(t)
	cases := []struct {
		h    *harness
		args []string
		code fail.Code
		fix  string
	}{
		{h, []string{"bogus"}, fail.Usage, "fix: run creds --help"},
		{h, []string{"get", "--nope", "web/mail"}, fail.Usage, "fix: run creds get --help"},
		{h, []string{"remote", "add"}, fail.Usage, "fix: run creds remote add --help"},
		{h, []string{"add", "--type", "bogus", "x"}, fail.Usage, "--type login|"},
		{h, []string{"resolve", "x"}, fail.Usage, "--mine or --theirs"},
		{h, []string{"get", "web/nope"}, fail.NotFound, "fix: run creds search web/nope\n"},
		{h, []string{"rm", "--yes", "it's"}, fail.NotFound, "fix: run " + shellLine([]string{"creds", "search", "it's"})},
		{h, []string{"get", "db/prod", "--as", "nope"}, fail.Usage, "fix: run creds get db/prod --formats\n"},
		{fresh, []string{"list"}, fail.NoVault, "fix: run creds init"},
		{h, []string{"remote", "remove"}, fail.General, "fix: run creds remote add <url>"},
	}
	for _, c := range cases {
		r := c.h.run(&fake{}, c.args...)
		if r.code != int(c.code) || !strings.HasPrefix(r.err, "error: ") || !strings.Contains(r.err, c.fix) {
			t.Errorf("%v: code %d err %q", c.args, r.code, r.err)
		}
	}
}

func TestNoTTYNeverPrompts(t *testing.T) {
	t.Parallel()
	h := seeded(t)
	locked := seeded(t)
	locked.expire()
	cases := []struct {
		h    *harness
		args []string
		code fail.Code
		fix  string
	}{
		{locked, []string{"get", "web/mail"}, fail.Locked, "fix: run creds unlock in a terminal first"},
		{h, []string{"rm", "web/mail"}, fail.Usage, "fix: run creds rm web/mail --yes\n"},
		{h, []string{"add", "web/new"}, fail.Usage, "fix: pass flags instead"},
		{h, []string{"get", "web/mail"}, fail.OK, ""},
	}
	for _, c := range cases {
		term, out := pipeTerminal(t, mainWord+"\ny\n")
		env, _, errb := c.h.env(&fake{})
		env.Prompter = term
		code := Run(env, c.args)
		if code != int(c.code) || !strings.Contains(errb.String(), c.fix) || out.Len() != 0 {
			t.Errorf("%v: code %d err %q prompt %q", c.args, code, errb.String(), out.String())
		}
	}
}

func TestYesSkipsConfirm(t *testing.T) {
	t.Parallel()
	h := seeded(t)
	file := filepath.Join(t.TempDir(), "x.json")
	h.ok(&fake{}, "export", "--plain", "-o", file, "--yes", "--confirm-plaintext", app.ExportPhrase)
	h.ok(&fake{}, "import", "--overwrite", "--yes", file)
	h.ok(&fake{confirms: []bool{true}}, "import", "--overwrite", file)
	r := h.run(&fake{confirms: []bool{false}}, "import", "--overwrite", file)
	if r.code != int(fail.General) || !strings.Contains(r.err, "aborted") {
		t.Fatalf("declined import: %d %q", r.code, r.err)
	}
	dir := t.TempDir()
	writeFile(t, dir, ".creds.toml", "[map]\nDB = \"db/prod|url\"\n")
	if r = h.run(&fake{confirms: []bool{false}}, "trust", dir); r.code == 0 {
		t.Fatalf("declined trust: %q", r.err)
	}
	h.ok(&fake{}, "trust", "--yes", dir)
	f := &fake{}
	h.run(f, "resolve", "x", "--mine", "--yes")
	h.run(f, "join", "--yes", filepath.Join(dir, "missing.git"))
	if len(f.prompts) != 0 {
		t.Fatalf("prompted %v", f.prompts)
	}
}

func TestFillHint(t *testing.T) {
	t.Parallel()
	root := NewRoot(Env{})
	cases := []struct {
		args []string
		err  error
		want string
	}{
		{[]string{"init"}, app.ErrPasswordMismatch, "run creds init again and type the same password twice"},
		{[]string{"recover"}, app.ErrWeakPassword, "run creds recover again with a stronger password, like four random words"},
		{[]string{"run", "--", "psql"}, vault.ErrNotFound, "run creds search <text>"},
		{[]string{"run", "db/x", "--", "psql"}, vault.ErrNotFound, "run creds search db/x"},
		{[]string{"trust", "dir"}, vault.ErrNotFound, "run creds search <text>"},
		{[]string{"trust", "--", "dir"}, errNeedYes, "run creds trust --yes -- dir"},
		{[]string{"init"}, errNeedYes, "run creds init in a terminal and answer the question"},
		{[]string{"sync"}, fmt.Errorf("%w: %w", gitsync.ErrConflict, &app.ConflictError{Paths: []string{"work/db-renamed"}}), "run creds resolve work/db-renamed --mine|--theirs"},
		{[]string{"sync"}, &app.ConflictError{Paths: []string{"a b", "c"}}, "run creds resolve 'a b' --mine|--theirs, then the same for the other conflicts"},
	}
	for _, c := range cases {
		cmd, rest, err := root.Find(c.args)
		if err != nil {
			t.Fatal(err)
		}
		if err := cmd.ParseFlags(rest); err != nil {
			t.Fatal(err)
		}
		got := fillHint(cmd, c.args, classify(cmd, runError{c.err}))
		if got.Hint != c.want {
			t.Errorf("%v: hint %q want %q", c.args, got.Hint, c.want)
		}
	}
}
