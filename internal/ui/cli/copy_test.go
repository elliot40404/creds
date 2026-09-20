package cli

import (
	"errors"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/elliot40404/creds/internal/app"
	"github.com/elliot40404/creds/internal/clipboard"
	"github.com/elliot40404/creds/internal/vault"
)

func TestDefaultFieldOrder(t *testing.T) {
	t.Parallel()
	pw := vault.Field{Name: app.FieldPassword, Value: "p", Secret: true}
	tok := vault.Field{Name: app.FieldToken, Value: "t", Secret: true}
	pin := vault.Field{Name: "pin", Value: "1", Secret: true}
	plain := vault.Field{Name: "region", Value: "eu"}
	cmd := vault.Field{Name: app.FieldCommand, Value: "make"}
	cases := []struct {
		name string
		e    vault.Entry
		want string
	}{
		{"password first", vault.Entry{Fields: []vault.Field{pin, tok, pw}}, app.FieldPassword},
		{"token next", vault.Entry{Fields: []vault.Field{plain, pin, tok}}, app.FieldToken},
		{"first secret", vault.Entry{Fields: []vault.Field{plain, pin}}, "pin"},
		{"command", vault.Entry{Type: vault.TypeCommand, Fields: []vault.Field{cmd}}, app.FieldCommand},
	}
	for _, c := range cases {
		got, err := app.DefaultField(c.e)
		if err != nil || got != c.want {
			t.Fatalf("%s: %q %v", c.name, got, err)
		}
	}
	none := vault.Entry{Type: vault.TypeLogin, Fields: []vault.Field{plain, cmd}}
	if _, err := app.DefaultField(none); !errors.Is(err, app.ErrNoSecret) {
		t.Fatalf("none: %v", err)
	}
}

func TestCopyDefaultField(t *testing.T) {
	t.Parallel()
	h := seeded(t)
	for path, want := range map[string]string{"web/mail": loginHidden, "db/prod": dbConn} {
		label := path + ".password"
		if path == "db/prod" {
			label = path + " as url"
		}
		h.clip.jobs = nil
		r := h.ok(&fake{}, "copy", path)
		if h.clip.value != want {
			t.Fatalf("%s: wrong value copied", path)
		}
		if r.out != "" || r.err != "Copied "+label+"\n" {
			t.Fatalf("%s: out %q err %q", path, r.out, r.err)
		}
		noLeak(t, r, loginHidden, dbHidden, extraHidden)
		job := h.clip.jobs[0]
		if len(h.clip.jobs) != 1 || job.Mode != clipboard.ModeNative || job.Hash != clipboard.Hash(want) || job.After != 30*time.Second {
			t.Fatalf("%s: jobs %+v", path, h.clip.jobs)
		}
	}
}

func TestCopyField(t *testing.T) {
	t.Parallel()
	h := seeded(t)
	r := h.ok(&fake{}, "cp", "db/prod", "--field", "token_ref")
	if h.clip.value != extraHidden || r.err != "Copied db/prod.token_ref\n" {
		t.Fatalf("err %q", r.err)
	}
	h.ok(&fake{}, "copy", "web/mail", "--field", "username")
	if h.clip.value != "alice" {
		t.Fatalf("value %q", h.clip.value)
	}
	r = h.fail(&fake{}, "copy", "web/mail", "--field", "nope")
	if !strings.Contains(r.err, "no such field") || strings.Contains(r.err, "--show") {
		t.Fatalf("err %q", r.err)
	}
}

func TestCopyFailureHint(t *testing.T) {
	t.Parallel()
	h := seeded(t)
	h.clip.writeErr = errors.New("no clipboard utilities")
	r := h.fail(&fake{}, "copy", "web/mail")
	want := "error: no clipboard utilities\nfix: run creds get web/mail --field password --show\n"
	if r.err != want || r.out != "" {
		t.Fatalf("err %q", r.err)
	}
	noLeak(t, r, loginHidden)
	if len(h.clip.jobs) != 0 {
		t.Fatal("clear child spawned after failure")
	}
}

func TestCopyFailureShowsValue(t *testing.T) {
	t.Parallel()
	h := seeded(t)
	h.clip.writeErr = errors.New("clipboard busy")
	h.clip.errTTY = true
	f := &fake{confirms: []bool{true}}
	r := h.ok(f, "copy", "db/prod")
	if r.out != dbConn+"\n" || !strings.Contains(r.err, "Shown db/prod as url") {
		t.Fatalf("out %q err %q", r.out, r.err)
	}
	if !strings.Contains(f.prompts[0], "clipboard busy") || strings.Contains(f.prompts[0], dbHidden) {
		t.Fatalf("prompt %q", f.prompts[0])
	}
	r = h.fail(&fake{confirms: []bool{false}}, "copy", "db/prod")
	if r.out != "" || !strings.Contains(r.err, "fix: run creds get db/prod --as url") {
		t.Fatalf("out %q err %q", r.out, r.err)
	}
	noLeak(t, r, dbHidden)
}

func TestCopyFailureNoTTYHidesValue(t *testing.T) {
	t.Parallel()
	h := seeded(t)
	h.clip.writeErr = errors.New("clipboard busy")
	f := &fake{confirms: []bool{true}}
	r := h.fail(f, "copy", "db/prod")
	if len(f.prompts) != 0 || !strings.Contains(r.err, "fix: run creds get db/prod --as url") {
		t.Fatalf("prompts %v err %q", f.prompts, r.err)
	}
	noLeak(t, r, dbHidden)
}

func TestCopyOSC52OverSSH(t *testing.T) {
	t.Parallel()
	h := seeded(t)
	h.clip.vars = map[string]string{"SSH_TTY": "/dev/pts/3"}
	h.clip.tty = filepath.Join(t.TempDir(), "tty")
	r := h.ok(&fake{}, "copy", "web/mail")
	got, err := os.ReadFile(h.clip.tty)
	if err != nil {
		t.Fatal(err)
	}
	want, _ := clipboard.Sequence(loginHidden, false)
	if string(got) != string(want) || h.clip.value != "" {
		t.Fatalf("tty %q clip %q", got, h.clip.value)
	}
	if r.err != "Copied web/mail.password\n" {
		t.Fatalf("err %q", r.err)
	}
	job := h.clip.jobs[0]
	if job.Mode != clipboard.ModeOSC52 || job.TTY == nil {
		t.Fatalf("job %+v", job)
	}
}

func TestCopyOSC52FallbackToStderr(t *testing.T) {
	t.Parallel()
	h := seeded(t)
	h.clip.vars = map[string]string{"TMUX": "/tmp/tmux-1/default,1,0"}
	h.clip.errTTY = true
	h.clip.spawnErr = clipboard.ErrNoTTY
	r := h.ok(&fake{}, "copy", "--osc52", "web/mail")
	want, _ := clipboard.Sequence(loginHidden, true)
	if !strings.HasPrefix(r.err, string(want)) || !strings.Contains(r.err, "Copied web/mail.password\n") {
		t.Fatalf("err %q", r.err)
	}
	if !strings.HasSuffix(r.err, "will not be cleared automatically: osc52 clear needs a terminal\n") || h.clip.jobs[0].TTY != nil {
		t.Fatalf("err %q jobs %+v", r.err, h.clip.jobs)
	}
	noLeak(t, r, loginHidden)
}

func TestCopyOSC52NoTerminal(t *testing.T) {
	t.Parallel()
	h := seeded(t)
	h.clip.vars = map[string]string{"SSH_CONNECTION": "a b c d"}
	r := h.fail(&fake{}, "copy", "web/mail")
	if strings.Contains(r.err, "]52;") || len(h.clip.jobs) != 0 || h.clip.value != "" {
		t.Fatalf("err %q jobs %+v", r.err, h.clip.jobs)
	}
	if !strings.Contains(r.err, "no terminal for OSC52") || !strings.Contains(r.err, "creds copy --native") {
		t.Fatalf("err %q", r.err)
	}
	noLeak(t, r, loginHidden)
}

func TestCopyForcedNativeOverSSH(t *testing.T) {
	t.Parallel()
	h := seeded(t)
	h.clip.vars = map[string]string{"SSH_CONNECTION": "a b c d"}
	h.ok(&fake{}, "copy", "--native", "web/mail")
	if h.clip.value != loginHidden || h.clip.jobs[0].Mode != clipboard.ModeNative {
		t.Fatalf("jobs %+v", h.clip.jobs)
	}
	h.fail(&fake{}, "copy", "--native", "--osc52", "web/mail")
}

func TestCopyCommandEntry(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.init()
	h.ok(&fake{inputs: []string{"make deploy", ""}}, "add", "--type", "command", "ops/deploy")
	r := h.ok(&fake{}, "copy", "ops/deploy")
	if h.clip.value != "make deploy" || r.err != "Copied ops/deploy.command\n" {
		t.Fatalf("value %q err %q", h.clip.value, r.err)
	}
}

func TestNoExecPath(t *testing.T) {
	t.Parallel()
	banned := []string{"os/exec", "os.StartProcess", "syscall.Exec", "syscall.ForkExec", "syscall.StartProcess"}
	for _, dir := range []string{".", "../../app", "../../vault", "../../render"} {
		files, err := filepath.Glob(filepath.Join(dir, "*.go"))
		if err != nil {
			t.Fatal(err)
		}
		for _, name := range files {
			if strings.HasSuffix(name, "_test.go") {
				continue
			}
			checkNoExec(t, name, banned)
		}
	}
}

func checkNoExec(t *testing.T, name string, banned []string) {
	t.Helper()
	f, err := parser.ParseFile(token.NewFileSet(), name, nil, parser.ImportsOnly)
	if err != nil {
		t.Fatal(err)
	}
	for _, imp := range f.Imports {
		if slices.Contains(banned, strings.Trim(imp.Path.Value, `"`)) {
			t.Fatalf("%s imports %s", name, imp.Path.Value)
		}
	}
	src, err := os.ReadFile(filepath.Clean(name))
	if err != nil {
		t.Fatal(err)
	}
	for _, b := range banned {
		if strings.Contains(string(src), b+"(") {
			t.Fatalf("%s calls %s", name, b)
		}
	}
}
