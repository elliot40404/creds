package tui

import (
	"errors"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/elliot40404/creds/internal/app"
	"github.com/elliot40404/creds/internal/sshdir"
)

func sshFile(rel string, kind sshdir.Kind, data string) *sshdir.File {
	return &sshdir.File{Rel: rel, Kind: kind, Data: []byte(data)}
}

func fakeSSH() app.SSHStatus {
	cfg := func(rel, data string) *sshdir.File { return sshFile(rel, sshdir.KindConfig, data) }
	key := func(rel, data string) *sshdir.File { return sshFile(rel, sshdir.KindPrivateKey, data) }
	return app.SSHStatus{
		Dir: "/home/me/.ssh",
		Files: []app.SSHFileStatus{
			{Rel: "config", Kind: sshdir.KindConfig, Change: app.SSHConflict, Disk: cfg("config", "Host a\n  User me\n"), Vault: cfg("config", "Host a\n  User you\n")},
			{Rel: "config.d/db", Kind: sshdir.KindConfig, Change: app.SSHLocalChanged, Disk: cfg("config.d/db", "Host db\n"), Vault: cfg("config.d/db", "Host old\n")},
			{Rel: "config.d/old", Kind: sshdir.KindConfig, Change: app.SSHRemoteDeleted, Disk: cfg("config.d/old", "Host old\n")},
			{Rel: "config.d/web", Kind: sshdir.KindConfig, Change: app.SSHRemoteNew, Vault: cfg("config.d/web", "Host web\n")},
			{Rel: "id_ed25519", Kind: sshdir.KindPrivateKey, Change: app.SSHConflict, Disk: key("id_ed25519", secretA), Vault: key("id_ed25519", secretB)},
			{Rel: "id_ed25519.pub", Kind: sshdir.KindPublicKey, Change: app.SSHSame, Disk: cfg("id_ed25519.pub", "ssh-ed25519 AAAA"), Vault: cfg("id_ed25519.pub", "ssh-ed25519 AAAA")},
			{Rel: "id_spare", Kind: sshdir.KindPrivateKey, Change: app.SSHLocalNew, Unused: true, Disk: key("id_spare", secretA)},
		},
		Missing:  []string{"id_gone"},
		Warnings: []error{errors.New("config line 4 uses %h, skipped")},
	}
}

func sshFake() *fakeBackend {
	f := newFake()
	f.ssh = fakeSSH()
	return f
}

func (f *fakeBackend) SSHStatus() (app.SSHStatus, error) {
	st := f.ssh
	st.Files = slices.Clone(f.ssh.Files)
	return st, f.sshErr
}

func (f *fakeBackend) SSHPush(o app.SSHPushOptions) (app.SSHPushResult, error) {
	f.pushes = append(f.pushes, o)
	return app.SSHPushResult{Files: f.sshApply(func(s app.SSHFileStatus) app.SSHAction {
		switch {
		case s.Change == app.SSHConflict && o.AskConflict != nil && asked(o.AskConflict(s.Rel)):
			return app.SSHPushed
		case s.Unused:
			return app.SSHNotAsked
		case s.Next() == app.SSHNeedsPush:
			return app.SSHPushed
		}
		return s.Next()
	})}, f.sshErr
}

func (f *fakeBackend) SSHPull(o app.SSHPullOptions) (app.SSHPullResult, error) {
	f.pulls = append(f.pulls, o)
	return app.SSHPullResult{Files: f.sshApply(func(s app.SSHFileStatus) app.SSHAction {
		switch {
		case s.Change == app.SSHRemoteDeleted && o.AskDelete != nil && asked(o.AskDelete(s)):
			return app.SSHDeleted
		case s.Change == app.SSHConflict && o.AskConflict != nil && asked(o.AskConflict(s)):
			return app.SSHUpdated
		case s.Change == app.SSHRemoteNew:
			return app.SSHCreated
		}
		return s.Next()
	})}, f.sshErr
}

func asked(ok bool, err error) bool {
	return ok && err == nil
}

func (f *fakeBackend) sshApply(decide func(app.SSHFileStatus) app.SSHAction) []app.SSHFileResult {
	var out []app.SSHFileResult
	kept := f.ssh.Files[:0:0]
	for _, s := range f.ssh.Files {
		a := decide(s)
		out = append(out, app.SSHFileResult{Rel: s.Rel, Change: s.Change, Action: a})
		switch a {
		case app.SSHDeleted:
			continue
		case app.SSHPushed, app.SSHCreated, app.SSHUpdated:
			s.Change = app.SSHSame
		}
		kept = append(kept, s)
	}
	f.ssh.Files = kept
	return out
}

func sshChange(m Model, rel string) app.SSHChange {
	s, ok := m.top().(sshScreen)
	if !ok {
		return 0
	}
	for _, f := range s.status.Files {
		if f.Rel == rel {
			return f.Change
		}
	}
	return 0
}

func TestSSHScreenOpensAndGoesBack(t *testing.T) {
	m := press(start(t, sshFake(), Hooks{}), ch('S'))
	if _, ok := m.top().(sshScreen); !ok || m.op.busy() {
		t.Fatalf("top %T busy %v", m.top(), m.op.busy())
	}
	v := view(m)
	for _, want := range []string{"ssh files", "6 of 7 files differ", "config.d/db", "local changed", "p pushes it", "unused key, not asked", "missing key file: id_gone"} {
		if !strings.Contains(v, want) {
			t.Errorf("view lacks %q:\n%s", want, v)
		}
	}
	noSecrets(t, m)
	if m = press(m, esc); !isList(m) || m.quit {
		t.Fatalf("esc should go back to the list, top %T", m.top())
	}
}

func isList(m Model) bool {
	_, ok := m.top().(listScreen)
	return ok
}

func TestSSHScreenEmpty(t *testing.T) {
	m := press(start(t, newFake(), Hooks{}), ch('S'))
	if v := view(m); !strings.Contains(v, "no ssh files here or in the vault") {
		t.Fatalf("view %q", v)
	}
	m = press(m, enter, ch('P'), ch('L'), down)
	if _, ok := m.top().(sshScreen); !ok {
		t.Fatalf("top %T", m.top())
	}
}

func TestSSHScreenLoadError(t *testing.T) {
	f := sshFake()
	f.sshErr = errors.New("vault is broken")
	m := press(start(t, f, Hooks{}), ch('S'))
	if !isList(m) || !strings.Contains(view(m), "vault is broken") || m.op.busy() {
		t.Fatalf("view %q", view(m))
	}
}

func TestSSHPushKeepsConflicts(t *testing.T) {
	f := sshFake()
	m := press(start(t, f, Hooks{}), ch('S'), ch('p'))
	if len(f.pushes) != 1 || f.pushes[0].AskConflict != nil || f.pushes[0].Overwrite || f.pushes[0].All {
		t.Fatalf("pushes %+v", f.pushes)
	}
	if v := view(m); !strings.Contains(v, "pushed 1 files, 3 left for you to pick") {
		t.Fatalf("view %q", v)
	}
	if sshChange(m, "config.d/db") != app.SSHSame || sshChange(m, "config") != app.SSHConflict {
		t.Fatalf("status not refreshed: %v", view(m))
	}
}

func TestSSHPullAsksNothing(t *testing.T) {
	f := sshFake()
	m := press(start(t, f, Hooks{}), ch('S'), ch('l'))
	if len(f.pulls) != 1 || f.pulls[0].AskConflict != nil || f.pulls[0].AskDelete != nil || f.pulls[0].Overwrite {
		t.Fatalf("pulls %+v", f.pulls)
	}
	if v := view(m); !strings.Contains(v, "pulled 1 files") {
		t.Fatalf("view %q", v)
	}
	if sshChange(m, "config.d/web") != app.SSHSame || sshChange(m, "config.d/old") != app.SSHRemoteDeleted {
		t.Fatal("pull should create web and keep old")
	}
}

func TestSSHPushFailure(t *testing.T) {
	f := sshFake()
	m := press(start(t, f, Hooks{}), ch('S'))
	f.sshErr = errors.New("disk on fire")
	m = press(m, ch('p'))
	if v := view(m); !strings.Contains(v, "disk on fire") || m.op.busy() {
		t.Fatalf("view %q", v)
	}
}

func TestSSHConflictKeepHereAsksFirst(t *testing.T) {
	f := sshFake()
	m := press(start(t, f, Hooks{}), ch('S'), ch('P'))
	if v := view(m); !strings.Contains(v, "Replace the vault copy of config with the file here?") || len(f.pushes) != 0 {
		t.Fatalf("view %q", v)
	}
	m = press(m, ch('n'))
	if _, ok := m.top().(sshScreen); !ok || len(f.pushes) != 0 {
		t.Fatalf("no should keep both, top %T", m.top())
	}
	m = press(m, ch('P'), ch('y'))
	if len(f.pushes) != 1 || sshChange(m, "config") != app.SSHSame || sshChange(m, "id_ed25519") != app.SSHConflict {
		t.Fatalf("pushes %d, view %q", len(f.pushes), view(m))
	}
	if ok, _ := f.pushes[0].AskConflict("id_ed25519"); ok {
		t.Fatal("only the picked file may be overwritten")
	}
}

func TestSSHTakeVaultCopyAndDelete(t *testing.T) {
	f := sshFake()
	m := press(start(t, f, Hooks{}), ch('S'), ch('L'))
	if v := view(m); !strings.Contains(v, "Replace config here with the vault copy?") {
		t.Fatalf("view %q", v)
	}
	m = press(m, ch('y'))
	if sshChange(m, "config") != app.SSHSame || sshChange(m, "id_ed25519") != app.SSHConflict {
		t.Fatalf("view %q", view(m))
	}
	m = press(m, down, ch('P'), ch('L'))
	if len(f.pulls) != 1 || len(f.pushes) != 0 {
		t.Fatal("P and L should do nothing on a file without a conflict")
	}
	m = press(m, down, ch('L'))
	if v := view(m); !strings.Contains(v, "Delete config.d/old here?") {
		t.Fatalf("view %q", v)
	}
	if m = press(m, ch('y')); sshChange(m, "config.d/old") != 0 || len(f.pulls) != 2 {
		t.Fatalf("delete failed, view %q", view(m))
	}
}

func TestSSHDiffView(t *testing.T) {
	m := press(start(t, sshFake(), Hooks{}), ch('S'), enter)
	v := view(m)
	for _, want := range []string{"config", "- here, + vault", "-   User me", "+   User you", "  Host a"} {
		if !strings.Contains(v, want) {
			t.Errorf("diff lacks %q:\n%s", want, v)
		}
	}
	if m = press(m, esc); sshChange(m, "config") != app.SSHConflict {
		t.Fatalf("esc should go back to the ssh screen, top %T", m.top())
	}
}

func TestSSHDiffNeverShowsKeys(t *testing.T) {
	m := press(start(t, sshFake(), Hooks{}), ch('S'), down, down, down, down, ch('d'))
	if v := view(m); !strings.Contains(v, "key differs, content not shown") {
		t.Fatalf("view %q", v)
	}
	noSecrets(t, m)
	m = press(m, esc, down, down, enter)
	if v := view(m); !strings.Contains(v, "id_spare") || !strings.Contains(v, "key differs") {
		t.Fatalf("view %q", v)
	}
	noSecrets(t, m)
}

func TestSSHDiffScrolls(t *testing.T) {
	f := sshFake()
	f.ssh.Files[0].Vault = sshFile("config", sshdir.KindConfig, strings.Repeat("Host x\n", 30)+"Host last\n")
	m := press(start(t, f, Hooks{}), ch('S'), enter)
	if strings.Contains(view(m), "Host last") {
		t.Fatal("the end should be below the fold")
	}
	for range 40 {
		m = press(m, down)
	}
	if !strings.Contains(view(m), "Host last") {
		t.Fatalf("view %q", view(m))
	}
	if m = press(m, up); strings.Contains(view(m), "Host last") {
		t.Fatal("one step up should scroll back at once")
	}
}

func TestSSHRefresh(t *testing.T) {
	f := sshFake()
	m := press(start(t, f, Hooks{}), ch('S'))
	f.ssh.Files = f.ssh.Files[:1]
	if m = press(m, ch('r')); !strings.Contains(view(m), "1 of 1 files differ") {
		t.Fatalf("view %q", view(m))
	}
}

func TestSSHScreenCleansPaths(t *testing.T) {
	f := sshFake()
	evil := "config\x1b]0;owned\a"
	f.ssh.Files[0].Rel = evil
	f.ssh.Missing = []string{"gone\x1b[2J"}
	m := press(start(t, f, Hooks{}), ch('S'))
	for _, k := range []tea.KeyPressMsg{ch('r'), ch('P'), esc, ch('L'), esc, enter} {
		m = press(m, k)
		if raw := m.View().Content; strings.Contains(raw, "\x1b]0;") || strings.Contains(raw, "\x1b[2J") {
			t.Fatalf("raw control text in view %q", raw)
		}
	}
}
