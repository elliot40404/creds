package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func sshStateBytes(t *testing.T, h *harness) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(h.home, "ssh.json"))
	if os.IsNotExist(err) {
		return ""
	}
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestSSHStatusListsChanges(t *testing.T) {
	t.Parallel()
	h, dir := sshHarness(t, "work_key")
	r := h.ok(&fake{noTTY: true}, "ssh", "status")
	if r.out != "local new      config\nlocal new      used_key\nlocal new      work_key\n" {
		t.Fatalf("out %q", r.out)
	}
	for _, want := range []string{"3 files changed here, run creds ssh push", "3 of 3 files differ"} {
		if !strings.Contains(r.err, want) {
			t.Fatalf("err %q, want %q", r.err, want)
		}
	}
	if sshStateBytes(t, h) != "" {
		t.Fatal("status wrote ssh.json")
	}
	h.ok(&fake{confirms: []bool{false}}, "ssh", "push")
	if r = h.ok(&fake{}, "ssh", "status"); r.out != "local new      work_key\n" || !strings.Contains(r.err, "1 unused keys skipped") {
		t.Fatalf("skipped %q %q", r.out, r.err)
	}
	h.ok(&fake{}, "rm", "ssh/used_key", "--yes")
	writeFile(t, dir, "config", "Host b\n  IdentityFile ~/.ssh/used_key\n")
	state := sshStateBytes(t, h)
	r = h.ok(&fake{}, "ssh", "status")
	if r.out != "local changed  config\nremote deleted used_key\nlocal new      work_key\n" {
		t.Fatalf("out %q", r.out)
	}
	for _, want := range []string{"run creds ssh push", "run creds ssh pull", "3 of 3 files differ"} {
		if !strings.Contains(r.err, want) {
			t.Fatalf("err %q, want %q", r.err, want)
		}
	}
	if sshStateBytes(t, h) != state {
		t.Fatal("status changed ssh.json")
	}
}

func TestSSHStatusJSONAndInteractive(t *testing.T) {
	t.Parallel()
	h, dir := pushedSSH(t)
	if r := h.ok(&fake{}, "ssh", "status"); r.out != "" || !strings.Contains(r.err, "2 files up to date") {
		t.Fatalf("in sync %q %q", r.out, r.err)
	}
	forgetSSHState(t, h)
	writeFile(t, dir, "config", "Host b\n")
	if err := os.Remove(filepath.Join(dir, "used_key")); err != nil {
		t.Fatal(err)
	}
	r := h.ok(&fake{noTTY: true}, "ssh", "status", "--json")
	var got jsonSSHRun
	if err := json.Unmarshal([]byte(r.out), &got); err != nil {
		t.Fatalf("%v: %q", err, r.out)
	}
	want := []jsonSSHFile{{Path: "config", Change: "conflict", Action: "conflict"}, {Path: "used_key", Change: "remote new", Action: "pull"}}
	if len(got.Files) != 2 || got.Files[0] != want[0] || got.Files[1] != want[1] {
		t.Fatalf("json %+v", got)
	}
	if sshStateBytes(t, h) != "" {
		t.Fatal("status wrote ssh.json")
	}
	r = h.ok(&fake{picks: []string{"status"}}, "ssh", "-i")
	if !strings.Contains(r.out, "conflict       config\n") || !strings.Contains(r.err, "command: creds ssh status") {
		t.Fatalf("%q %q", r.out, r.err)
	}
}
