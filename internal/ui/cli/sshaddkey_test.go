package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/elliot40404/creds/internal/fail"
)

func TestSSHAddKeyCopiesOutsideKey(t *testing.T) {
	t.Parallel()
	h, dir := sshHarness(t)
	out := t.TempDir()
	key := sshKeyPEM(t)
	writeFile(t, out, "work.pem", key)
	writeFile(t, out, "work.pem.pub", "junk\n")
	r := h.ok(&fake{}, "ssh", "add-key", filepath.Join(out, "work.pem"), "--name", "work_key")
	if r.out != "created      work_key\n" || !strings.Contains(r.err, "left out work_key.pub: not a public key") ||
		!strings.Contains(r.err, "add IdentityFile ~/.ssh/work_key to a host or run creds ssh add-host") {
		t.Fatalf("%q %q", r.out, r.err)
	}
	data, err := os.ReadFile(filepath.Clean(filepath.Join(dir, "work_key")))
	if err != nil || string(data) != key {
		t.Fatalf("disk %v", err)
	}
	if r = h.ok(&fake{}, "get", "ssh/work_key", "--field", "content", "--show"); !strings.HasPrefix(r.out, key) {
		t.Fatalf("vault %q", r.out)
	}
	r = h.ok(&fake{}, "ssh", "add-key", filepath.Join(out, "work.pem"), "--name", "work_key", "--json")
	var got jsonSSHAddKey
	if err := json.Unmarshal([]byte(r.out), &got); err != nil {
		t.Fatalf("%v: %q", err, r.out)
	}
	if got.Path != "work_key" || got.Dir != dir || !got.Unused || len(got.Files) != 1 || got.Files[0].Action != "same" {
		t.Fatalf("json %+v", got)
	}
}

func TestSSHAddKeyPushesSkippedKey(t *testing.T) {
	t.Parallel()
	h, dir := sshHarness(t, "spare_key")
	h.ok(&fake{noTTY: true}, "ssh", "push")
	if r := h.ok(&fake{}, "ssh", "status"); !strings.Contains(r.out, "spare_key") {
		t.Fatalf("status %q", r.out)
	}
	r := h.ok(&fake{}, "ssh", "add-key", filepath.Join(dir, "spare_key"))
	if r.out != "pushed       spare_key\n" || !strings.Contains(r.err, "not used by ~/.ssh/config yet") {
		t.Fatalf("%q %q", r.out, r.err)
	}
	if r = h.ok(&fake{}, "ssh", "status"); strings.Contains(r.out, "spare_key") {
		t.Fatalf("still differs %q", r.out)
	}
	r = h.ok(&fake{}, "ssh", "add-key", filepath.Join(dir, "used_key"))
	if r.out != "" || !strings.Contains(r.err, "used_key is already in ~/.ssh and the vault") || strings.Contains(r.err, "not used") {
		t.Fatalf("used %q %q", r.out, r.err)
	}
}

func TestSSHAddKeyRefuses(t *testing.T) {
	t.Parallel()
	h, dir := sshHarness(t)
	out := t.TempDir()
	writeFile(t, out, "id.pub", "ssh-ed25519 AAAA\n")
	writeFile(t, out, "used_key", sshKeyPEM(t))
	for _, args := range [][]string{{filepath.Join(out, "id.pub")}, {filepath.Join(dir, "config")}, {filepath.Join(out, "used_key"), "--name", "a/b"}} {
		if r := h.fail(&fake{}, append([]string{"ssh", "add-key"}, args...)...); r.code != int(fail.Usage) || !strings.Contains(r.err, "creds ssh add-key") {
			t.Errorf("%v: %d %q", args, r.code, r.err)
		}
	}
	if r := h.fail(&fake{}, "ssh", "add-key", filepath.Join(out, "used_key")); r.code != int(fail.Conflict) {
		t.Fatalf("existing: %d %q", r.code, r.err)
	}
}

func TestSSHAddKeyInteractive(t *testing.T) {
	t.Parallel()
	h, dir := sshHarness(t)
	out := t.TempDir()
	writeFile(t, out, "k", sshKeyPEM(t))
	f := &fake{inputs: []string{filepath.Join(out, "k"), "deploy_key"}}
	r := h.ok(f, "ssh", "add-key", "-i")
	if len(f.prompts) != 2 || !strings.Contains(r.err, "command: creds ssh add-key") || !strings.Contains(r.err, "--name deploy_key") {
		t.Fatalf("%q %q", f.prompts, r.err)
	}
	if _, err := os.Stat(filepath.Join(dir, "deploy_key")); err != nil {
		t.Fatal(err)
	}
	f = &fake{}
	h.ok(f, "ssh", "add-key", filepath.Join(dir, "used_key"), "--name", "used_key", "-i")
	if len(f.prompts) != 0 {
		t.Fatalf("asked %q", f.prompts)
	}
}
