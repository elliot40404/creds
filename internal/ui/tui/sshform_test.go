package tui

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/elliot40404/creds/internal/app"
	"github.com/elliot40404/creds/internal/sshdir"
)

func (f *fakeBackend) SSHAddHost(h app.SSHHost) (app.SSHAddHostResult, error) {
	f.sshAdds = append(f.sshAdds, fmt.Sprintf("host %s %s %s %d %s", h.Name, h.Host, h.User, h.Port, h.Key))
	if f.addErr != nil {
		return app.SSHAddHostResult{}, f.addErr
	}
	res := app.SSHAddHostResult{Dir: f.ssh.Dir, Rel: h.Rel(), Alias: h.Alias()}
	if strings.Contains(h.Name, "/") {
		res.Include = h.IncludeLine()
	}
	f.sshAdded(res.Rel, sshdir.KindConfig)
	return res, nil
}

func (f *fakeBackend) SSHAddKey(file, name string) (app.SSHAddKeyResult, error) {
	f.sshAdds = append(f.sshAdds, "key "+file+" "+name)
	if f.addErr != nil {
		return app.SSHAddKeyResult{}, f.addErr
	}
	f.sshAdded(name, sshdir.KindPrivateKey)
	return app.SSHAddKeyResult{Rel: name, Files: []app.SSHFileResult{{Rel: name, Action: app.SSHCreated}}, Unused: true}, nil
}

func (f *fakeBackend) SSHKeygen(k app.SSHKeygen) (app.SSHKeygenResult, error) {
	f.sshAdds = append(f.sshAdds, fmt.Sprintf("keygen %s %s %v %q", k.Name, k.Comment, k.Passphrase, k.PassphraseBytes))
	if f.addErr != nil {
		return app.SSHKeygenResult{}, f.addErr
	}
	f.sshAdded(k.Name, sshdir.KindPrivateKey)
	return app.SSHKeygenResult{Rel: k.Name}, nil
}

func (f *fakeBackend) sshAdded(rel string, kind sshdir.Kind) {
	f.ssh.Files = append(slices.Clone(f.ssh.Files), app.SSHFileStatus{Rel: rel, Kind: kind, Change: app.SSHSame, Disk: sshFile(rel, kind, "x"), Vault: sshFile(rel, kind, "x")})
}

func sshFormOpen(t *testing.T, f *fakeBackend, k rune) Model {
	t.Helper()
	m := press(start(t, f, Hooks{}), ch('S'), ch(k))
	if _, ok := m.top().(sshForm); !ok {
		t.Fatalf("top %T", m.top())
	}
	return m
}

func backOnSSH(t *testing.T, m Model, flash string) {
	t.Helper()
	if _, ok := m.top().(sshScreen); !ok || m.op.busy() || m.failed || !strings.Contains(m.flash, flash) {
		t.Fatalf("top %T busy %v flash %q", m.top(), m.op.busy(), m.flash)
	}
}

func TestSSHAddHostForm(t *testing.T) {
	f := sshFake()
	m := sshFormOpen(t, f, 'a')
	if v := view(m); !strings.Contains(v, "Add ssh host") || !strings.Contains(v, "like id_ed25519, id_spare") {
		t.Fatalf("view %q", v)
	}
	m = press(m, ctrl('s'))
	if !strings.Contains(view(m), "name is required") || len(f.sshAdds) != 0 {
		t.Fatalf("view %q added %q", view(m), f.sshAdds)
	}
	m = press(m, text("beam/data_new")...)
	m = press(m, tabKey)
	m = press(m, text("10.0.0.5")...)
	m = press(m, tabKey)
	m = press(m, text("me")...)
	m = press(m, tabKey)
	m = press(m, text("22x")...)
	m = press(m, ctrl('s'))
	if !strings.Contains(view(m), "port must be a number") || len(f.sshAdds) != 0 {
		t.Fatalf("view %q", view(m))
	}
	m = press(m, back, tabKey)
	m = press(m, text("id_ed25519")...)
	m = press(m, enter)
	if !slices.Equal(f.sshAdds, []string{"host beam/data_new 10.0.0.5 me 22 id_ed25519"}) {
		t.Fatalf("added %q", f.sshAdds)
	}
	backOnSSH(t, m, "created config.d/beam/data_new, add to ~/.ssh/config: Include config.d/*/*")
	if !strings.Contains(view(m), "config.d/beam/data_new") {
		t.Fatalf("status not reloaded %q", view(m))
	}
}

func TestSSHAddFormShowsAppError(t *testing.T) {
	f := sshFake()
	f.addErr = fmt.Errorf("%w: config.d/db is already in ~/.ssh", app.ErrSSHExists)
	m := sshFormOpen(t, f, 'a')
	m = press(m, text("db")...)
	m = press(m, tabKey)
	m = press(m, text("h")...)
	m = press(m, ctrl('s'))
	v := view(m)
	if _, ok := m.top().(sshForm); !ok || m.op.busy() || !strings.Contains(v, "already in ~/.ssh") || !strings.Contains(v, "pick another name") {
		t.Fatalf("top %T view %q", m.top(), v)
	}
}

func TestSSHAddKeyForm(t *testing.T) {
	f := sshFake()
	m := sshFormOpen(t, f, 'i')
	m = press(m, enter, enter)
	if !strings.Contains(view(m), "file is required") || len(f.sshAdds) != 0 {
		t.Fatalf("view %q", view(m))
	}
	m = press(m, text("/tmp/id")...)
	m = press(m, tabKey)
	m = press(m, text("work")...)
	m = press(m, enter)
	if !slices.Equal(f.sshAdds, []string{"key /tmp/id work"}) {
		t.Fatalf("added %q", f.sshAdds)
	}
	backOnSSH(t, m, "added work, no host uses it yet, press a to add one")
}

func TestSSHKeyFormExpandsHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	for in, want := range map[string]string{"~/dl/id": filepath.Join(home, "dl", "id"), "~": home, "~x/id": "~x/id", "/tmp/~/id": "/tmp/~/id"} {
		if got := homePath(in); got != want {
			t.Fatalf("%s: got %q want %q", in, got, want)
		}
	}
}

func TestSSHKeygenForm(t *testing.T) {
	f := sshFake()
	m := sshFormOpen(t, f, 'g')
	m = press(m, text("id_work")...)
	m = press(m, tabKey, tabKey, ch(' '))
	if v := view(m); !strings.Contains(v, "repeat") || !strings.Contains(v, "< on >") {
		t.Fatalf("view %q", v)
	}
	m = press(m, tabKey)
	m = press(m, text(secretA)...)
	m = press(m, tabKey)
	m = press(m, text(secretB)...)
	m = press(m, enter)
	if !strings.Contains(view(m), "passphrases do not match") || len(f.sshAdds) != 0 || m.top().(sshForm).focus != keygenPass {
		t.Fatalf("view %q added %q", view(m), f.sshAdds)
	}
	noSecrets(t, m)
	m = press(m, text(secretA)...)
	m = press(m, tabKey)
	m = press(m, text(secretA)...)
	m = press(m, ctrl('s'))
	if want := fmt.Sprintf("keygen id_work  true %q", secretA); !slices.Equal(f.sshAdds, []string{want}) {
		t.Fatalf("added %q", f.sshAdds)
	}
	backOnSSH(t, m, "created id_work and id_work.pub")
	noSecrets(t, m)
}

func TestSSHKeygenEmptyPassphrase(t *testing.T) {
	f := sshFake()
	m := sshFormOpen(t, f, 'g')
	m = press(m, text("id_work")...)
	m = press(m, tabKey, tabKey, rightKey, ctrl('s'))
	if !strings.Contains(view(m), "passphrase is empty") || len(f.sshAdds) != 0 {
		t.Fatalf("view %q", view(m))
	}
	m = press(m, up, ch(' '), ctrl('s'))
	if !slices.Equal(f.sshAdds, []string{`keygen id_work  false ""`}) {
		t.Fatalf("added %q", f.sshAdds)
	}
	backOnSSH(t, m, "created id_work")
}

func TestSSHFormsEscCancel(t *testing.T) {
	for _, k := range []rune{'a', 'i', 'g'} {
		f := sshFake()
		m := press(sshFormOpen(t, f, k), ch('x'), esc)
		if _, ok := m.top().(sshScreen); !ok || len(f.sshAdds) != 0 {
			t.Fatalf("%c: top %T added %q", k, m.top(), f.sshAdds)
		}
	}
}
