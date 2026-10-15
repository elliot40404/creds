package cli

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/elliot40404/creds/internal/fsutil"
)

func readSSH(t *testing.T, dir, name string) string {
	t.Helper()
	data, err := fsutil.ReadFileLimit(filepath.Join(dir, name), 1<<16)
	if errors.Is(err, fs.ErrNotExist) {
		return ""
	}
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func pushedSSH(t *testing.T) (*harness, string) {
	t.Helper()
	h, dir := sshHarness(t)
	h.ok(&fake{}, "ssh", "push")
	return h, dir
}

func forgetSSHState(t *testing.T, h *harness) {
	t.Helper()
	if err := os.Remove(filepath.Join(h.home, "ssh.json")); err != nil {
		t.Fatal(err)
	}
}

func TestSSHPullRestoresANewMachine(t *testing.T) {
	t.Parallel()
	h, dir := pushedSSH(t)
	key := readSSH(t, dir, "used_key")
	forgetSSHState(t, h)
	if err := os.RemoveAll(dir); err != nil {
		t.Fatal(err)
	}
	r := h.ok(&fake{noTTY: true}, "ssh", "pull")
	if r.out != "created      config\ncreated      used_key\n" || !strings.Contains(r.err, "pulled 2 files") {
		t.Fatalf("%q %q", r.out, r.err)
	}
	if readSSH(t, dir, "config") != "Host a\n  IdentityFile ~/.ssh/used_key\n" || readSSH(t, dir, "used_key") != key {
		t.Fatal("files not restored")
	}
	if r = h.ok(&fake{noTTY: true}, "ssh", "pull"); r.out != "" || !strings.Contains(r.err, "nothing to pull") {
		t.Fatalf("second pull %q %q", r.out, r.err)
	}
	if r = h.ok(&fake{}, "ssh", "push"); !strings.Contains(r.err, "nothing to push") {
		t.Fatalf("push after pull %q %q", r.out, r.err)
	}
}

func TestSSHPullConflictShowsDiff(t *testing.T) {
	t.Parallel()
	h, dir := pushedSSH(t)
	forgetSSHState(t, h)
	writeFile(t, dir, "config", "Host b\n  IdentityFile ~/.ssh/used_key\n")
	writeFile(t, dir, "used_key", sshKeyPEM(t))
	r := h.ok(&fake{noTTY: true}, "ssh", "pull")
	if r.out != "conflict     config\nconflict     used_key\n" || !strings.Contains(r.err, "creds ssh pull -y") {
		t.Fatalf("no tty %q %q", r.out, r.err)
	}
	f := &fake{confirms: []bool{false, false}}
	r = h.ok(f, "ssh", "pull")
	for _, want := range []string{"config (- here, + vault):", "  - Host b\n", "  + Host a\n", "    IdentityFile", "used_key: key differs"} {
		if !strings.Contains(r.err, want) {
			t.Fatalf("diff %q, want %q", r.err, want)
		}
	}
	if strings.Contains(r.err, "PRIVATE KEY") || len(f.prompts) != 2 || !strings.Contains(f.prompts[0], "Overwrite the local copy?") {
		t.Fatalf("err %q prompts %q", r.err, f.prompts)
	}
	if r = h.ok(&fake{noTTY: true}, "ssh", "pull", "-y"); r.out != "updated      config\nupdated      used_key\n" {
		t.Fatalf("yes %q", r.out)
	}
	if readSSH(t, dir, "config") != "Host a\n  IdentityFile ~/.ssh/used_key\n" {
		t.Fatal("config not overwritten")
	}
}

func TestSSHPullAsksBeforeDelete(t *testing.T) {
	t.Parallel()
	h, dir := pushedSSH(t)
	h.ok(&fake{}, "rm", "ssh/config", "--yes")
	r := h.ok(&fake{noTTY: true}, "ssh", "pull", "-y")
	if r.out != "not asked    config\n" || !strings.Contains(r.err, "creds ssh pull in a terminal") || readSSH(t, dir, "config") == "" {
		t.Fatalf("no tty %q %q", r.out, r.err)
	}
	if r = h.ok(&fake{confirms: []bool{false}}, "ssh", "pull"); r.out != "kept         config\n" || readSSH(t, dir, "config") == "" {
		t.Fatalf("declined %q", r.out)
	}
	f := &fake{confirms: []bool{true}}
	if r = h.ok(f, "ssh", "pull"); r.out != "deleted      config\n" || readSSH(t, dir, "config") != "" {
		t.Fatalf("deleted %q", r.out)
	}
	if len(f.prompts) != 1 || f.prompts[0] != "Delete config? It was deleted in the vault." {
		t.Fatalf("prompts %q", f.prompts)
	}
}

func TestSSHPullJSONAndInteractive(t *testing.T) {
	t.Parallel()
	h, dir := pushedSSH(t)
	if err := os.Remove(filepath.Join(dir, "config")); err != nil {
		t.Fatal(err)
	}
	r := h.ok(&fake{noTTY: true}, "ssh", "pull", "--json")
	var got jsonSSHRun
	if err := json.Unmarshal([]byte(r.out), &got); err != nil {
		t.Fatalf("%v: %q", err, r.out)
	}
	if len(got.Files) != 2 || got.Files[0] != (jsonSSHFile{Path: "config", Change: "local deleted", Action: "push"}) {
		t.Fatalf("json %+v", got)
	}
	forgetSSHState(t, h)
	writeFile(t, dir, "config", "Host b\n")
	r = h.ok(&fake{picks: []string{"pull"}, confirms: []bool{true}}, "ssh", "-i")
	if !strings.Contains(r.out, "updated      config\n") || !strings.Contains(r.err, "command: creds ssh pull") {
		t.Fatalf("%q %q", r.out, r.err)
	}
	h.fail(&fake{noTTY: true}, "ssh", "pull", "-i")
}
