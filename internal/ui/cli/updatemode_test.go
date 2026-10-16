package cli

import (
	"strings"
	"testing"
)

func TestConfigUpdateMode(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	if r := h.ok(&fake{}, "config", "get", "update.mode"); strings.TrimSpace(r.out) != "hint" {
		t.Fatalf("default %q", r.out)
	}
	if r := h.ok(&fake{}, "config"); !strings.Contains(r.out, "update.mode") {
		t.Fatalf("update.mode not listed %q", r.out)
	}
	h.ok(&fake{}, "config", "set", "update.mode", "off")
	if r := h.ok(&fake{}, "config", "get", "update.mode"); strings.TrimSpace(r.out) != "off" {
		t.Fatalf("after set %q", r.out)
	}
	r := h.fail(&fake{}, "config", "set", "update.mode", "always")
	if !strings.Contains(r.err, "not an update mode") || !strings.Contains(r.err, "hint, install, or off") {
		t.Fatalf("err %q", r.err)
	}
}
