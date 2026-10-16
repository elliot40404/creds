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
