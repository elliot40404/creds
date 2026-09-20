package cli

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/elliot40404/creds/internal/app"
	"github.com/elliot40404/creds/internal/gitsync"
)

type fakeHost struct {
	url     string
	created []string
	err     error
}

func (h *fakeHost) Available(context.Context) error { return h.err }

func (h *fakeHost) Create(_ context.Context, name string) (string, error) {
	h.created = append(h.created, name)
	return h.url, nil
}

func (h *fakeHost) Repos(context.Context) ([]string, error) { return []string{h.url}, nil }

func found(names ...string) func(string) (string, error) {
	return func(file string) (string, error) {
		if slices.Contains(names, file) {
			return filepath.Join("/usr/bin", file), nil
		}
		return "", errors.New("not found")
	}
}

func hook(look func(string) (string, error), host app.Hoster) func(*app.Setup) {
	return func(u *app.Setup) {
		u.OS = "linux"
		if look != nil {
			u.Look = look
		}
		u.Host = host
	}
}

func (h *harness) setup(f *fake, set func(*app.Setup), args ...string) result {
	h.t.Helper()
	env, out, errb := h.env(f)
	env.Setup = set
	code := Run(env, args)
	return result{code: code, out: out.String(), err: errb.String()}
}

func (h *harness) setupOK(f *fake, set func(*app.Setup), args ...string) result {
	h.t.Helper()
	r := h.setup(f, set, args...)
	if r.code != 0 {
		h.t.Fatalf("%v: code %d stderr %q", args, r.code, r.err)
	}
	if !f.drained() {
		h.t.Fatalf("%v: unused answers %+v", args, f)
	}
	return r
}

func newVault(picks ...string) *fake {
	return &fake{
		picks:     append([]string{"Create a new vault"}, picks...),
		passwords: []string{mainWord, mainWord},
		inputs:    []string{shownCode},
	}
}

func TestSetupNoVaultNoTTYPrintsSteps(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	r := h.setup(&fake{noTTY: true}, hook(found("git"), nil))
	if r.code != 0 {
		t.Fatalf("code %d %q", r.code, r.err)
	}
	for _, want := range []string{"creds setup", "creds init", "creds join <url>", "ok   git"} {
		if !strings.Contains(r.out, want) {
			t.Fatalf("out %q missing %q", r.out, want)
		}
	}
}

func TestSetupNewVaultNoRemote(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	f := newVault("Skip sync for now")
	f.confirms = []bool{false}
	r := h.setupOK(f, hook(found("git", "wl-copy"), nil), "setup")
	if !strings.Contains(r.out, "vault created") || !strings.Contains(r.out, "creds add -i") {
		t.Fatalf("out = %q", r.out)
	}
	h.ok(&fake{}, "list")
}

func TestSetupNewVaultPushesToURL(t *testing.T) {
	t.Parallel()
	remote := bareRemote(t)
	h := newHarness(t)
	f := newVault("Paste a git url")
	f.inputs = append(f.inputs, remote)
	f.confirms = []bool{true, false}
	r := h.setupOK(f, hook(found("git"), nil), "setup")
	if !strings.Contains(r.out, "vault pushed to") {
		t.Fatalf("out = %q", r.out)
	}
	if out := h.ok(&fake{}, "status").out; !strings.Contains(out, remote) {
		t.Fatalf("status = %q", out)
	}
}

func TestSetupNewVaultUsesGH(t *testing.T) {
	t.Parallel()
	host := &fakeHost{url: bareRemote(t)}
	h := newHarness(t)
	f := newVault("Create a private GitHub repo with gh")
	f.inputs = append(f.inputs, "creds-vault")
	f.confirms = []bool{true, true, false}
	r := h.setupOK(f, hook(found("git", "gh"), host), "setup")
	if len(host.created) != 1 || host.created[0] != "creds-vault" {
		t.Fatalf("created = %v", host.created)
	}
	if !strings.Contains(r.out, "vault pushed to") {
		t.Fatalf("out = %q", r.out)
	}
}

func TestSetupGHOptionHiddenWithoutGH(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	f := newVault("Create a private GitHub repo with gh")
	f.confirms = []bool{false}
	r := h.setup(f, hook(found("git"), nil), "setup")
	if r.code == 0 || !strings.Contains(r.err, "not in") {
		t.Fatalf("code %d err %q", r.code, r.err)
	}
}

func seedVaultRemote(t *testing.T) string {
	t.Helper()
	remote := bareRemote(t)
	a := newHarness(t)
	a.init()
	a.addCommand("ops/deploy", "make deploy")
	a.ok(&fake{}, "remote", "add", remote)
	a.ok(&fake{}, "sync")
	return remote
}

func TestSetupKeepsVaultWhenRemoteHasVault(t *testing.T) {
	t.Parallel()
	remote := seedVaultRemote(t)
	h := newHarness(t)
	f := newVault("Paste a git url", "Skip sync for now")
	f.inputs = append(f.inputs, remote)
	f.confirms = []bool{false}
	r := h.setupOK(f, hook(found("git"), nil), "setup")
	if !strings.Contains(r.out, "vault kept, pick another remote or skip sync") {
		t.Fatalf("out %q missing %q", r.out, "vault kept, pick another remote or skip sync")
	}
	if !strings.Contains(strings.Join(f.warned, " "), "join") {
		t.Fatalf("warned %v", f.warned)
	}
	if slices.ContainsFunc(f.prompts, func(p string) bool { return strings.HasPrefix(p, "Push the vault to") }) {
		t.Fatalf("offered a push to a remote with a vault: %v", f.prompts)
	}
	h.ok(&fake{}, "list")
}

func TestSetupAsksAgainWhenRepoMissing(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	f := newVault("Paste a git url", "Skip sync for now")
	f.inputs = append(f.inputs, filepath.Join(t.TempDir(), "gone.git"))
	f.confirms = []bool{false}
	r := h.setupOK(f, hook(found("git"), nil), "setup")
	if !strings.Contains(r.out, "vault kept, pick another remote or skip sync") {
		t.Fatalf("out %q missing %q", r.out, "vault kept, pick another remote or skip sync")
	}
	if !strings.Contains(strings.Join(f.warned, " "), "repository not found") {
		t.Fatalf("warned %v", f.warned)
	}
}

func seedJunk(t *testing.T, remote string) {
	t.Helper()
	ctx := context.Background()
	work := filepath.Join(t.TempDir(), "work")
	if _, err := gitsync.NewGit(filepath.Dir(work)).Run(ctx, "clone", "-q", "--", remote, work); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(work, "README.md"), []byte("hi"), 0o600); err != nil {
		t.Fatal(err)
	}
	g := gitsync.NewGit(work)
	for _, args := range [][]string{{"add", "-A"}, {"commit", "-q", "-m", "seed"}, {"push", "-q", "origin", "HEAD:" + gitsync.Branch}} {
		if _, err := g.Run(ctx, args...); err != nil {
			t.Fatal(err)
		}
	}
}

func TestSetupKeepsVaultWhenRemoteHasJunk(t *testing.T) {
	t.Parallel()
	junk := bareRemote(t)
	seedJunk(t, junk)
	h := newHarness(t)
	f := newVault("Paste a git url", "Skip sync for now")
	f.inputs = append(f.inputs, junk)
	f.confirms = []bool{false}
	r := h.setupOK(f, hook(found("git"), nil), "setup")
	if !strings.Contains(r.out, "vault kept, pick another remote or skip sync") {
		t.Fatalf("out %q missing %q", r.out, "vault kept, pick another remote or skip sync")
	}
	if !strings.Contains(strings.Join(f.warned, " "), "not empty") {
		t.Fatalf("warned %v", f.warned)
	}
}

func TestSetupJoinsExistingVault(t *testing.T) {
	t.Parallel()
	remote := seedVaultRemote(t)
	h := newHarness(t)
	f := &fake{
		picks:     []string{"Join a vault that already exists"},
		inputs:    []string{remote},
		passwords: []string{mainWord},
	}
	r := h.setupOK(f, hook(found("git"), nil), "setup")
	if !strings.Contains(r.out, "1 entry") {
		t.Fatalf("out = %q", r.out)
	}
	if out := h.ok(&fake{}, "get", "ops/deploy").out; !strings.Contains(out, "make deploy") {
		t.Fatalf("get = %q", out)
	}
}

func TestSetupJoinRefusesRemoteWithoutVault(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	bare := bareRemote(t)
	f := &fake{
		picks:  []string{"Join a vault that already exists"},
		inputs: slices.Repeat([]string{bare}, app.MaxTries),
	}
	r := h.setup(f, hook(found("git"), nil), "setup")
	if r.code == 0 || !strings.Contains(r.err, "no creds vault") {
		t.Fatalf("code %d err %q", r.code, r.err)
	}
}

func TestSetupShowsPreflightFixes(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	f := newVault("Skip sync for now")
	f.confirms = []bool{false}
	r := h.setupOK(f, hook(found(), nil), "setup")
	if !strings.Contains(r.out, "fix  git") || !strings.Contains(r.out, "fix  clipboard") {
		t.Fatalf("out = %q", r.out)
	}
}

func TestSetupSavesSettings(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	f := newVault("Skip sync for now")
	f.confirms = []bool{true}
	f.inputs = append(f.inputs, "5m", "2h")
	f.picks = append(f.picks, "bash")
	h.setupOK(f, hook(found("git"), nil), "setup")
	data, err := os.ReadFile(filepath.Join(h.home, "config.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "5m0s") || !strings.Contains(string(data), "bash") {
		t.Fatalf("config = %q", data)
	}
}

func TestSetupWithVaultShowsNextSteps(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.init()
	f := &fake{picks: []string{"Show next steps"}}
	r := h.setupOK(f, hook(found("git"), nil), "setup")
	if !strings.Contains(r.out, "creds add -i") {
		t.Fatalf("out %q", r.out)
	}
}

func TestSetupWithVaultChangesRemote(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.init()
	h.ok(&fake{}, "remote", "add", bareRemote(t))
	next := bareRemote(t)
	f := &fake{picks: []string{"Change the git remote", "Paste a git url"}, inputs: []string{next}, confirms: []bool{true}}
	r := h.setupOK(f, hook(found("git"), nil), "setup")
	if !strings.Contains(r.out, "vault pushed to "+next) {
		t.Fatalf("out %q", r.out)
	}
	st := h.ok(&fake{}, "status")
	if !strings.Contains(st.out, next) {
		t.Fatalf("status %q", st.out)
	}
}

func TestSetupWithVaultChangesSettings(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.init()
	f := &fake{picks: []string{"Change settings", "bash"}, confirms: []bool{true}, inputs: []string{"5m", "2h"}}
	h.setupOK(f, hook(found("git"), nil), "setup")
	data, err := os.ReadFile(filepath.Join(h.home, "config.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "5m0s") {
		t.Fatalf("config %q", data)
	}
}

func TestSetupLeavesVaultUnlocked(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	f := newVault("Skip sync for now")
	f.confirms = []bool{false}
	h.setupOK(f, hook(found("git"), nil), "setup")
	h.ok(&fake{}, "list")
}

func TestSetupPushPromptHidesURLToken(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	f := newVault("Paste a git url")
	f.inputs = append(f.inputs, "https://user:SECRETTOKEN@example.invalid/x.git")
	f.confirms = []bool{false, false}
	set := func(u *app.Setup) {
		hook(found("git"), nil)(u)
		u.Probe = func(context.Context, string) (app.RemoteState, error) { return app.RemoteEmpty, nil }
	}
	r := h.setupOK(f, set, "setup")
	all := strings.Join(f.prompts, "\n") + r.out + r.err
	if strings.Contains(all, "SECRETTOKEN") {
		t.Fatalf("token shown: %q", all)
	}
	if !slices.Contains(f.prompts, "Push the vault to https://example.invalid/x.git?") {
		t.Fatalf("prompts = %v", f.prompts)
	}
}

func TestSetupJoinPicksAHostRepo(t *testing.T) {
	t.Parallel()
	remote := seedVaultRemote(t)
	h := newHarness(t)
	f := &fake{
		picks:     []string{"Join a vault that already exists", remote},
		passwords: []string{mainWord},
	}
	r := h.setupOK(f, hook(found("git"), &fakeHost{url: remote}), "setup")
	if !strings.Contains(r.out, "1 entry") {
		t.Fatalf("out = %q", r.out)
	}
}
