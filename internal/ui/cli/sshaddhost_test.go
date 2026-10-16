package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/elliot40404/creds/internal/fail"
)

func TestSSHAddHost(t *testing.T) {
	t.Parallel()
	h, dir := sshHarness(t)
	args := []string{"ssh", "add-host", "beam/data_new", "--host", "10.0.0.5", "--user", "ubuntu", "--port", "2222", "--key", "used_key"}
	r := h.ok(&fake{}, args...)
	if r.out != "created      config.d/beam/data_new\n" || !strings.Contains(r.err, "add this line at the top of it: Include config.d/*/*") {
		t.Fatalf("%q %q", r.out, r.err)
	}
	want := "Host data_new\n  HostName 10.0.0.5\n  User ubuntu\n  Port 2222\n  IdentityFile ~/.ssh/used_key\n  IdentitiesOnly yes\n"
	data, err := os.ReadFile(filepath.Clean(filepath.Join(dir, "config.d", "beam", "data_new")))
	if err != nil || string(data) != want {
		t.Fatalf("disk %q %v", data, err)
	}
	if r = h.ok(&fake{}, "get", "ssh/config.d/beam/data_new", "--field", "content"); !strings.HasPrefix(r.out, want) {
		t.Fatalf("vault %q", r.out)
	}
	r = h.fail(&fake{}, args...)
	if r.code != int(fail.Conflict) || !strings.Contains(r.err, "creds ssh push") {
		t.Fatalf("again: %d %q", r.code, r.err)
	}
	for _, bad := range [][]string{{"x", "--host", "h", "--port", "70000"}, {"a/b/c", "--host", "h"}, {"x"}, {"x", "--host", "h", "--key", "nope"}} {
		if r = h.fail(&fake{}, append([]string{"ssh", "add-host"}, bad...)...); r.code != int(fail.Usage) {
			t.Errorf("%v: %d %q", bad, r.code, r.err)
		}
	}
}

func TestSSHAddHostIncludedJSON(t *testing.T) {
	t.Parallel()
	h, dir := sshHarness(t)
	writeFile(t, dir, "config", "Include config.d/*\n")
	r := h.ok(&fake{}, "ssh", "add-host", "db", "--host", "db.example.com", "--json")
	var got jsonSSHAddHost
	if err := json.Unmarshal([]byte(r.out), &got); err != nil {
		t.Fatalf("%v: %q", err, r.out)
	}
	if got.Path != "config.d/db" || got.Alias != "db" || got.Include != "" || got.Dir != dir {
		t.Fatalf("json %+v", got)
	}
	if r = h.ok(&fake{}, "ssh", "status"); strings.Contains(r.out, "config.d/db") || !strings.Contains(r.err, "2 of 3 files differ") {
		t.Fatalf("status %q %q", r.out, r.err)
	}
}

func TestSSHAddHostInteractiveAsksOnlyMissing(t *testing.T) {
	t.Parallel()
	h, dir := sshHarness(t)
	f := &fake{inputs: []string{"work/api", "api.example.com", "", ""}}
	r := h.ok(f, "ssh", "add-host", "-i", "--user", "deploy")
	if slices.ContainsFunc(f.prompts, func(p string) bool { return strings.HasPrefix(p, "User") }) || len(f.prompts) != 4 {
		t.Fatalf("prompts %q", f.prompts)
	}
	if !strings.Contains(r.err, "command: creds ssh add-host work/api --host api.example.com --user deploy") {
		t.Fatalf("err %q", r.err)
	}
	data, err := os.ReadFile(filepath.Clean(filepath.Join(dir, "config.d", "work", "api")))
	if err != nil || string(data) != "Host api\n  HostName api.example.com\n  User deploy\n" {
		t.Fatalf("disk %q %v", data, err)
	}
	f = &fake{}
	h.ok(f, "ssh", "add-host", "full", "--host", "h", "--user", "u", "--port", "22", "--key", "used_key", "-i")
	if len(f.prompts) != 0 {
		t.Fatalf("asked %q", f.prompts)
	}
	h.fail(&fake{noTTY: true}, "ssh", "add-host", "x", "-i")
}
