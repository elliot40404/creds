package cli

import (
	"strings"
	"testing"
)

func TestSSHAutoPushOnCommandStart(t *testing.T) {
	t.Parallel()
	h, dir := sshHarness(t)
	h.ok(&fake{noTTY: true}, "ssh", "push")
	writeFile(t, dir, "config", "Host a\n  IdentityFile ~/.ssh/used_key\n  Port 2222\n")
	if r := h.ok(&fake{}, "ssh", "status"); !strings.Contains(r.out, "local changed") {
		t.Fatalf("ssh status pushed on start: %q", r.out)
	}
	r := h.ok(&fake{}, "get", "ssh/config", "--field", "content")
	if !strings.Contains(r.out, "Port 2222") || r.err != "" {
		t.Fatalf("out %q err %q", r.out, r.err)
	}
	if r = h.ok(&fake{}, "ssh", "status"); r.out != "" || !strings.Contains(r.err, "up to date") {
		t.Fatalf("status after auto push %q %q", r.out, r.err)
	}
}

func TestSSHAutoConfigKey(t *testing.T) {
	t.Parallel()
	h, dir := sshHarness(t)
	if r := h.ok(&fake{}, "config", "get", "ssh.auto"); strings.TrimSpace(r.out) != "false" {
		t.Fatalf("before push %q", r.out)
	}
	h.ok(&fake{noTTY: true}, "ssh", "push")
	if r := h.ok(&fake{}, "config", "get", "ssh.auto"); strings.TrimSpace(r.out) != "true" {
		t.Fatalf("after push %q", r.out)
	}
	h.ok(&fake{}, "config", "set", "ssh.auto", "false")
	if r := h.ok(&fake{}, "config"); !strings.Contains(r.out, "ssh.auto") {
		t.Fatalf("config list %q", r.out)
	}
	writeFile(t, dir, "config", "Host a\n  IdentityFile ~/.ssh/used_key\n  Port 2222\n")
	r := h.ok(&fake{}, "get", "ssh/config", "--field", "content")
	if strings.Contains(r.out, "Port 2222") {
		t.Fatalf("pushed with ssh.auto false: %q", r.out)
	}
	h.ok(&fake{noTTY: true}, "ssh", "push")
	if r := h.ok(&fake{}, "config", "get", "ssh.auto"); strings.TrimSpace(r.out) != "false" {
		t.Fatalf("push turned it back on %q", r.out)
	}
}
