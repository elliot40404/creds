package cli

import (
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/elliot40404/creds/internal/fail"
	"github.com/elliot40404/creds/internal/vault"
)

func (h *harness) stdin(input string, args ...string) result {
	h.t.Helper()
	env, out, errb := h.env(&fake{})
	env.In = strings.NewReader(input)
	code := Run(env, args)
	return result{code: code, out: out.String(), err: errb.String()}
}

func TestAddByFlags(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.init()
	r := h.stdin(loginHidden+"\r\n"+extraHidden+"\n", "add", "web/app", "--type", "login", "--name", "App",
		"--username", "bob", "--url", "https://app.example", "--notes", "hi", "--tag", "a", "--tag", "b",
		"--field", "region=eu", "--secret-field", "password", "--secret-field", "pin=-")
	if r.code != 0 {
		t.Fatalf("add: %d %q", r.code, r.err)
	}
	noLeak(t, r, loginHidden, extraHidden)
	got := h.ok(&fake{}, "get", "web/app", "--show").out
	for _, want := range []string{"bob", "https://app.example", "hi", "region", "eu", loginHidden, extraHidden} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in %q", want, got)
		}
	}
	if masked := h.ok(&fake{}, "get", "web/app").out; strings.Contains(masked, extraHidden) {
		t.Fatalf("pin not secret: %q", masked)
	}
	r = h.stdin(dbConn+"\n", "add", "db/x", "--type", "database", "--engine", "postgres", "--conn", "-")
	if r.code != 0 {
		t.Fatalf("db add: %d %q", r.code, r.err)
	}
	if out := h.ok(&fake{}, "get", "db/x", "--as", "url").out; out != dbConn+"\n" {
		t.Fatalf("conn %q", out)
	}
}

func TestEditByFlags(t *testing.T) {
	t.Parallel()
	h := seeded(t)
	r := h.stdin(nextWord+"\n", "edit", "web/mail", "--username", "carol", "--secret-field", "password", "--field", "region=us")
	if r.code != 0 {
		t.Fatalf("edit: %d %q", r.code, r.err)
	}
	got := h.ok(&fake{}, "get", "web/mail", "--show").out
	if !strings.Contains(got, "carol") || !strings.Contains(got, nextWord) || !strings.Contains(got, "https://mail.example") {
		t.Fatalf("edited %q", got)
	}
	if r = h.stdin(dbConn+"\n", "edit", "db/prod", "--conn", "-"); r.code != 0 {
		t.Fatalf("db edit: %d %q", r.code, r.err)
	}
}

func TestSecretInArgsRejected(t *testing.T) {
	t.Parallel()
	h := seeded(t)
	for _, args := range [][]string{
		{"add", "x/a", "--type", "api", "--secret-field", "key=" + loginHidden},
		{"add", "x/b", "--type", "database", "--engine", "postgres", "--conn", dbConn},
		{"add", "x/c", "--type", "login", "--field", "password=" + loginHidden},
		{"edit", "db/prod", "--field", "token_ref=" + loginHidden},
		{"add", "x/d", "--type", "api", "--field", "novalue"},
		{"add", "x/e", "--username", "u"},
		{"add", "x/f", "--type", "api", "--secret-field", "key"},
		{"add", "x/g", "--type", "login", "--conn", "-"},
		{"add", "x/h", "--type", "database", "--engine", "postgres"},
		{"add", "x/i", "--type", "database", "--conn", "-"},
	} {
		r := h.stdin("", args...)
		if r.code != int(fail.Usage) || !strings.Contains(r.err, "fix: ") {
			t.Errorf("%v: %d %q", args, r.code, r.err)
		}
		noLeak(t, r, loginHidden, dbHidden)
	}
	if r := h.run(&fake{}, "get", "x/a"); r.code != int(fail.NotFound) {
		t.Fatalf("entry written: %d", r.code)
	}
}

func TestConnQueryPasswordHidden(t *testing.T) {
	t.Parallel()
	h := seeded(t)
	const conn = "postgres://bob@db.example/app?password=QS3CRET&sslpassword=K3YPASS&sslmode=require"
	if r := h.stdin(conn+"\n", "add", "db/q", "--type", "database", "--engine", "postgres", "--conn", "-"); r.code != 0 {
		t.Fatalf("add: %d %q", r.code, r.err)
	}
	for _, args := range [][]string{{"get", "db/q"}, {"get", "db/q", "--json"}} {
		r := h.ok(&fake{}, args...)
		noLeak(t, r, "QS3CRET", "K3YPASS")
		if !strings.Contains(r.out, "require") {
			t.Fatalf("%v: %q", args, r.out)
		}
	}
	if out := h.ok(&fake{}, "get", "db/q", "--as", "url").out; !strings.Contains(out, "bob:QS3CRET@") || !strings.Contains(out, "sslpassword=K3YPASS") {
		t.Fatalf("url %q", out)
	}
}

func TestLegacySecretParamMasked(t *testing.T) {
	t.Parallel()
	e := vault.Entry{Path: "db/old", Type: vault.TypeDatabase, Params: map[string]string{"password": "OLDS3CRET", "sslmode": "require"}}
	var b strings.Builder
	if err := writeEntry(&b, e, false); err != nil || strings.Contains(b.String(), "OLDS3CRET") || !strings.Contains(b.String(), "require") {
		t.Fatalf("text %q %v", b.String(), err)
	}
	if got := toJSONEntry(e, false).Params; !maps.Equal(got, map[string]string{"sslmode": "require"}) {
		t.Fatalf("json %v", got)
	}
	if got := toJSONEntry(e, true).Params; got["password"] != "OLDS3CRET" {
		t.Fatalf("json show %v", got)
	}
}

func TestSecretFieldRules(t *testing.T) {
	t.Parallel()
	h := seeded(t)
	for _, name := range []string{"notes", "username", "host", "url"} {
		r := h.stdin("S3CRET\n", "edit", "web/mail", "--secret-field", name)
		if r.code != int(fail.Usage) || !strings.Contains(r.err, "builtin") {
			t.Fatalf("%s: %d %q", name, r.code, r.err)
		}
	}
	h.ok(&fake{}, "edit", "web/mail", "--field", "token=visible")
	if r := h.stdin("S3CRET-TOKEN\n", "edit", "web/mail", "--secret-field", "token"); r.code != 0 {
		t.Fatalf("edit: %d %q", r.code, r.err)
	}
	r := h.ok(&fake{}, "get", "web/mail")
	noLeak(t, r, "S3CRET")
	if !strings.Contains(h.ok(&fake{}, "get", "web/mail", "--json").out, `{"name":"token","secret":true}`) {
		t.Fatal("token not secret")
	}
}

func TestSecretSourceTerminalNoEcho(t *testing.T) {
	t.Parallel()
	f := &fake{passwords: []string{"typed"}}
	v, err := secretSource(strings.NewReader("piped\n"), f, true)("pin")
	if err != nil || v != "typed" || !slices.Equal(f.prompts, []string{"pin"}) {
		t.Fatalf("tty %q %v %q", v, err, f.prompts)
	}
	v, err = secretSource(strings.NewReader("piped\n"), f, false)("pin")
	if err != nil || v != "piped" || len(f.prompts) != 1 {
		t.Fatalf("pipe %q %v", v, err)
	}
	if isTerminal(strings.NewReader("")) {
		t.Fatal("reader is not a terminal")
	}
}
