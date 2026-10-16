package tui

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/elliot40404/creds/internal/app"
	"github.com/elliot40404/creds/internal/sshdir"
)

func twoUnusedKeys() *fakeBackend {
	f := sshFake()
	f.ssh.Files = append(f.ssh.Files, app.SSHFileStatus{Rel: "id_work", Kind: sshdir.KindPrivateKey, Change: app.SSHLocalNew, Unused: true, Disk: sshFile("id_work", sshdir.KindPrivateKey, secretB)})
	return f
}

func isPicker(m Model) bool {
	_, ok := m.top().(sshKeyPicker)
	return ok
}

func TestSSHPushPicksUnusedKeys(t *testing.T) {
	f := twoUnusedKeys()
	m := press(start(t, f, Hooks{}), ch('S'), ch('p'))
	if !isPicker(m) || len(f.pushes) != 0 || !strings.Contains(view(m), "[ ] id_spare") {
		t.Fatalf("pushes %d view %q", len(f.pushes), view(m))
	}
	noSecrets(t, m)
	m = press(m, ch(' '))
	if v := view(m); !strings.Contains(v, "[x] id_spare") || !strings.Contains(v, "[ ] id_work") {
		t.Fatalf("view %q", v)
	}
	m = press(m, enter)
	slices.Sort(f.keyAnswers)
	if len(f.pushes) != 1 || !slices.Equal(f.keyAnswers, []string{"id_spare yes", "id_work no"}) {
		t.Fatalf("pushes %d answers %q", len(f.pushes), f.keyAnswers)
	}
	if v := view(m); sshChange(m, "id_spare") != app.SSHSame || !strings.Contains(v, "skipped, u pushes it") || !strings.Contains(v, "pushed 2 files") {
		t.Fatalf("view %q", v)
	}
	m = press(m, ch('p'))
	if isPicker(m) || len(f.pushes) != 2 || f.pushes[1].AskKey != nil {
		t.Fatalf("second push should not ask, top %T pushes %d", m.top(), len(f.pushes))
	}
}

func TestSSHKeyPickerAllAndCancel(t *testing.T) {
	f := twoUnusedKeys()
	m := press(start(t, f, Hooks{}), ch('S'), ch('p'), ch('a'))
	if v := view(m); !strings.Contains(v, "[x] id_spare") || !strings.Contains(v, "[x] id_work") {
		t.Fatalf("view %q", v)
	}
	m = press(m, down, ch(' '), ch('a'))
	if v := view(m); !strings.Contains(v, "[x] id_spare") || !strings.Contains(v, "[x] id_work") {
		t.Fatalf("a with one off should pick all: %q", v)
	}
	m = press(m, ch('a'))
	if v := view(m); strings.Contains(v, "[x]") {
		t.Fatalf("a with all on should pick none: %q", v)
	}
	m = press(m, esc)
	if _, ok := m.top().(sshScreen); !ok || len(f.pushes) != 0 || len(f.keyAnswers) != 0 || m.op.busy() {
		t.Fatalf("esc should cancel, top %T pushes %d", m.top(), len(f.pushes))
	}
}

func TestSSHPushWithoutNewKeysSkipsPicker(t *testing.T) {
	f := sshFake()
	f.ssh.Files[6].Skipped = true
	m := press(start(t, f, Hooks{}), ch('S'), ch('p'))
	if isPicker(m) || len(f.pushes) != 1 || len(f.keyAnswers) != 0 {
		t.Fatalf("top %T pushes %d answers %q", m.top(), len(f.pushes), f.keyAnswers)
	}
}

func TestSSHPushSkippedKey(t *testing.T) {
	f := sshFake()
	f.ssh.Files[6].Skipped = true
	m := press(start(t, f, Hooks{}), ch('S'), ch('u'))
	if len(f.sshAdds) != 0 {
		t.Fatalf("u on a non skipped row added %q", f.sshAdds)
	}
	m = press(m, down, down, down, down, down, down)
	if !strings.Contains(view(m), "unused key, skipped, u pushes it") {
		t.Fatalf("view %q", view(m))
	}
	m = press(m, ch('u'))
	if want := "key " + filepath.Join("/home/me/.ssh", "id_spare") + " "; !slices.Equal(f.sshAdds, []string{want}) {
		t.Fatalf("added %q", f.sshAdds)
	}
	if sshChange(m, "id_spare") != app.SSHSame || !strings.Contains(m.flash, "pushed 1 files") || m.op.busy() {
		t.Fatalf("flash %q view %q", m.flash, view(m))
	}
}
