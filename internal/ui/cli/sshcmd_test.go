package cli

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"encoding/pem"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/crypto/ssh"
)

func sshHarness(t *testing.T, extraKeys ...string) (*harness, string) {
	t.Helper()
	h := newHarness(t)
	h.init()
	dir := filepath.Join(h.user, ".ssh")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	_, sk, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	block, err := ssh.MarshalPrivateKey(sk, "")
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, dir, "config", "Host a\n  IdentityFile ~/.ssh/used_key\n")
	for _, name := range append([]string{"used_key"}, extraKeys...) {
		writeFile(t, dir, name, string(pem.EncodeToMemory(block)))
	}
	return h, dir
}

func TestSSHPushWithoutTerminalListsSkippedKeys(t *testing.T) {
	t.Parallel()
	h, _ := sshHarness(t, "work_key")
	r := h.ok(&fake{noTTY: true}, "ssh", "push")
	for _, want := range []string{"pushed       config\n", "pushed       used_key\n", "not asked    work_key\n"} {
		if !strings.Contains(r.out, want) {
			t.Fatalf("out %q, want %q", r.out, want)
		}
	}
	if !strings.Contains(r.err, "1 unused keys not pushed") || !strings.Contains(r.err, "--all") {
		t.Fatalf("err %q", r.err)
	}
	if r = h.ok(&fake{}, "get", "ssh/config", "--field", "content"); !strings.HasPrefix(r.out, "Host a\n  IdentityFile ~/.ssh/used_key\n") {
		t.Fatalf("vault config %q", r.out)
	}
	h.fail(&fake{}, "get", "ssh/work_key")
	r = h.ok(&fake{noTTY: true}, "ssh", "push", "--json")
	var got jsonSSHPush
	if err := json.Unmarshal([]byte(r.out), &got); err != nil {
		t.Fatalf("%v: %q", err, r.out)
	}
	actions := map[string]string{}
	for _, f := range got.Files {
		actions[f.Path] = f.Action + "/" + f.Change
	}
	if len(actions) != 3 || actions["config"] != "same/same" || actions["work_key"] != "not asked/local new" {
		t.Fatalf("json %+v", got)
	}
}

func TestSSHPushPickerSavesSkips(t *testing.T) {
	t.Parallel()
	h, _ := sshHarness(t, "work_key", "old_key")
	f := &fake{confirms: []bool{false, true}}
	r := h.ok(f, "ssh", "push")
	if !strings.Contains(r.out, "skipped      old_key\n") || !strings.Contains(r.out, "pushed       work_key\n") {
		t.Fatalf("out %q", r.out)
	}
	if len(f.prompts) != 2 || !strings.Contains(f.prompts[0], "old_key, a key no ssh config uses") {
		t.Fatalf("prompts %q", f.prompts)
	}
	if r = h.ok(&fake{}, "ssh", "push"); r.out != "skipped      old_key\n" || !strings.Contains(r.err, "nothing to push") {
		t.Fatalf("second push %q %q", r.out, r.err)
	}
	if r = h.ok(&fake{}, "ssh", "push", "--all"); r.out != "pushed       old_key\n" {
		t.Fatalf("all %q", r.out)
	}
}

func TestSSHPushConflictNeedsYes(t *testing.T) {
	t.Parallel()
	h, dir := sshHarness(t)
	h.ok(&fake{}, "ssh", "push")
	if err := os.Remove(filepath.Join(h.home, "ssh.json")); err != nil {
		t.Fatal(err)
	}
	writeFile(t, dir, "config", "Host b\n")
	r := h.ok(&fake{noTTY: true}, "ssh", "push")
	if r.out != "conflict     config\n" || !strings.Contains(r.err, "creds ssh push -y") {
		t.Fatalf("no tty %q %q", r.out, r.err)
	}
	if r = h.ok(&fake{confirms: []bool{false}}, "ssh", "push"); r.out != "conflict     config\n" {
		t.Fatalf("declined %q", r.out)
	}
	if r = h.ok(&fake{noTTY: true}, "ssh", "push", "-y"); r.out != "pushed       config\n" {
		t.Fatalf("yes %q", r.out)
	}
	if r = h.ok(&fake{}, "get", "ssh/config", "--field", "content"); !strings.HasPrefix(r.out, "Host b\n") {
		t.Fatalf("vault config %q", r.out)
	}
}

func TestSSHPushEscapesNames(t *testing.T) {
	t.Parallel()
	h, dir := sshHarness(t)
	writeFile(t, dir, "config", "IdentityFile ~/.ssh/used_key\nIdentityFile ~/.ssh/x\x1b[2J\n")
	r := h.ok(&fake{}, "ssh", "push")
	if strings.ContainsRune(r.out+r.err, '\x1b') || !strings.Contains(r.err, "missing key file") {
		t.Fatalf("raw escape: %q %q", r.out, r.err)
	}
}

func TestSSHPushInteractive(t *testing.T) {
	t.Parallel()
	h, _ := sshHarness(t, "work_key")
	r := h.ok(&fake{picks: []string{"push"}, confirms: []bool{true}}, "ssh", "-i")
	if !strings.Contains(r.out, "pushed       work_key\n") || !strings.Contains(r.err, "command: creds ssh push") {
		t.Fatalf("%q %q", r.out, r.err)
	}
	h.fail(&fake{noTTY: true}, "ssh", "push", "-i")
}
