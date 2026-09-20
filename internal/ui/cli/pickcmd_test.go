package cli

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/elliot40404/creds/internal/app"
	"github.com/elliot40404/creds/internal/crypto"
	"github.com/elliot40404/creds/internal/testutil"
	"github.com/elliot40404/creds/internal/ui/tui"
	"github.com/spf13/cobra"
)

type fakePicker struct {
	f       *fake
	choice  string
	seen    []string
	prompts int
	label   string
	err     error
}

func (p *fakePicker) picker() picker {
	return picker{
		interactive: func() bool { return true },
		open: func(opts tui.Options) error {
			p.prompts = len(p.f.prompts)
			items, err := opts.Backend.List()
			if err != nil {
				return err
			}
			for _, it := range items {
				p.seen = append(p.seen, it.Path)
			}
			if p.choice == "" {
				return nil
			}
			p.label, p.err = opts.Copy(p.choice, "", "", "")
			return p.err
		},
	}
}

func (h *harness) pick(f *fake, pk picker, args ...string) result {
	h.t.Helper()
	env, out, errb := h.env(f)
	root := &cobra.Command{Use: "creds", SilenceUsage: true, SilenceErrors: true}
	root.SetOut(env.Out)
	root.SetErr(env.Err)
	root.AddCommand(pickCmdWith(env, root, pk))
	root.SetArgs(append([]string{}, args...))
	code := 0
	if err := root.Execute(); err != nil {
		code = 1
	}
	return result{code: code, out: out.String(), err: errb.String()}
}

func TestPickNonTTYPrintsHelp(t *testing.T) {
	h := seeded(t)
	for _, args := range [][]string{{}, {"pick"}} {
		r := h.ok(&fake{}, args...)
		if !strings.Contains(r.out, "Usage:") || h.clip.value != "" {
			t.Fatalf("%v: out %q", args, r.out)
		}
	}
}

func TestPickUnlocksThenCopies(t *testing.T) {
	h := seeded(t)
	for _, args := range [][]string{{}, {"pick"}} {
		h.expire()
		h.clip.value = ""
		f := &fake{passwords: []string{mainWord}}
		fp := &fakePicker{f: f, choice: "web/mail"}
		r := h.pick(f, fp.picker(), args...)
		if r.code != 0 || !f.drained() || fp.prompts != 1 {
			t.Fatalf("%v: code %d err %q prompts %d", args, r.code, r.err, fp.prompts)
		}
		if strings.Join(fp.seen, ",") != "db/prod,web/mail" || h.clip.value != loginHidden {
			t.Fatalf("%v: seen %v", args, fp.seen)
		}
		if fp.label != "web/mail.password" || r.err != "" {
			t.Fatalf("%v: label %q err %q", args, fp.label, r.err)
		}
		noLeak(t, r, loginHidden, dbHidden, extraHidden)
	}
}

func TestPickCancelCopiesNothing(t *testing.T) {
	h := seeded(t)
	fp := &fakePicker{f: &fake{}}
	r := h.pick(fp.f, fp.picker(), []string{}...)
	if r.code != 0 || r.err != "" || h.clip.value != "" || len(h.clip.jobs) != 0 {
		t.Fatalf("code %d err %q", r.code, r.err)
	}
}

func TestPickCopyFailureGivesFix(t *testing.T) {
	h := seeded(t)
	h.clip.writeErr = errors.New("no clipboard utilities")
	f := &fake{}
	fp := &fakePicker{f: f, choice: "web/mail"}
	r := h.pick(f, fp.picker())
	want := "no clipboard utilities"
	if r.code != 1 || fp.err == nil || fp.err.Error() != want || len(h.clip.jobs) != 0 {
		t.Fatalf("code %d err %v", r.code, fp.err)
	}
	noLeak(t, r, loginHidden)
}

func TestPickClearFailureStaysOffStderr(t *testing.T) {
	h := seeded(t)
	h.clip.spawnErr = errors.New("no detach")
	f := &fake{}
	fp := &fakePicker{f: f, choice: "web/mail"}
	r := h.pick(f, fp.picker())
	if r.code != 0 || r.err != "" || h.clip.value != loginHidden {
		t.Fatalf("code %d err %q", r.code, r.err)
	}
	if fp.label != "web/mail.password, not cleared automatically: no detach" {
		t.Fatalf("label %q", fp.label)
	}
}

func TestPickRejectsArgs(t *testing.T) {
	h := seeded(t)
	fp := &fakePicker{f: &fake{}, choice: "web/mail"}
	if r := h.pick(fp.f, fp.picker(), "web/mail"); r.code != 1 || h.clip.value != "" {
		t.Fatalf("code %d", r.code)
	}
}

func TestPickWiresHooksAndRelock(t *testing.T) {
	h := seeded(t)
	f := &fake{}
	var steps []string
	pk := picker{
		interactive: func() bool { return true },
		open: func(opts tui.Options) error {
			if opts.Hooks.Add == nil || opts.Hooks.Edit == nil || opts.Hooks.Prompt == nil {
				t.Fatal("hooks not wired")
			}
			h.expire()
			_, err := opts.Copy("web/mail", "", "", "")
			steps = append(steps, fmt.Sprint(errors.Is(err, tui.ErrNeedPassword)))
			steps = append(steps, fmt.Sprint(errors.Is(opts.Unlock("wrong"), crypto.ErrWrongSecret)))
			steps = append(steps, fmt.Sprint(opts.Unlock(mainWord)))
			_, err = opts.Copy("web/mail", "", "", "")
			steps = append(steps, fmt.Sprint(err))
			return nil
		},
	}
	r := h.pick(f, pk)
	if r.code != 0 || strings.Join(steps, ",") != "true,true,<nil>,<nil>" || len(f.prompts) != 0 {
		t.Fatalf("code %d steps %v prompts %v", r.code, steps, f.prompts)
	}
	if h.clip.value != loginHidden {
		t.Fatal("copy after unlock failed")
	}
	noLeak(t, r, loginHidden)
}

func TestSetupUIOpensTUIWhenInteractive(t *testing.T) {
	h := newHarness(t)
	env, _, _ := h.env(&fake{})
	s, err := env.service(env.Prompter)
	if err != nil {
		t.Fatal(err)
	}
	u := s.Setup()
	var got tui.SetupOptions
	pk := picker{
		interactive: func() bool { return true },
		openSetup:   func(opts tui.SetupOptions) error { got = opts; return nil },
	}
	if err := env.setupWith(u, pk); err != nil {
		t.Fatal(err)
	}
	if got.Setup != u {
		t.Fatalf("opts %+v", got)
	}
}

func TestSetupUIFallsBackToCLI(t *testing.T) {
	h := newHarness(t)
	f := &fake{}
	env, out, _ := h.env(f)
	s, err := env.service(f)
	if err != nil {
		t.Fatal(err)
	}
	opened := false
	pk := picker{
		interactive: func() bool { return false },
		openSetup:   func(tui.SetupOptions) error { opened = true; return nil },
	}
	if err := env.setupWith(s.Setup(), pk); !errors.Is(err, testutil.ErrNoAnswer) {
		t.Fatalf("err %v", err)
	}
	if opened || !strings.Contains(out.String(), "home") {
		t.Fatalf("opened %v out %q", opened, out.String())
	}
}

func TestPickDoesNotSpawnSyncChildren(t *testing.T) {
	t.Parallel()
	h := seeded(t)
	n := 0
	env, _, errb := h.env(&fake{})
	env.Spawn = func() error {
		n++
		return nil
	}
	var got *app.Service
	pk := picker{
		interactive: func() bool { return true },
		open: func(o tui.Options) error {
			got, _ = o.Backend.(*app.Service)
			return nil
		},
	}
	svc, err := env.service(env.Prompter)
	if err != nil {
		t.Fatal(err)
	}
	if err := env.pick(svc, pk, tui.DefaultLayout()); err != nil {
		t.Fatalf("%v %q", err, errb.String())
	}
	if got == nil {
		t.Fatal("no service reached the TUI")
	}
	if got.Spawn != nil {
		t.Fatal("the TUI service still spawns a sync child per write")
	}
	if n != 0 {
		t.Fatalf("spawned %d children", n)
	}
}
