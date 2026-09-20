package cli

import (
	"strings"
	"testing"
)

const brokenConfig = "[session]\nidle = \"0s\"\n"

func TestBadConfigKeepsRepairCommands(t *testing.T) {
	t.Parallel()
	h := seeded(t)
	h.writeConfig(brokenConfig)
	for _, args := range [][]string{{"lock"}, {"paths"}, {"config"}, {"config", "get", "session.idle"}} {
		r := h.ok(&fake{}, args...)
		if !strings.Contains(r.err, "using default settings") || !strings.Contains(r.err, "session.idle") {
			t.Fatalf("%v warned %q", args, r.err)
		}
	}
}

func TestBadConfigCanBeFixedBySet(t *testing.T) {
	t.Parallel()
	h := seeded(t)
	h.writeConfig(brokenConfig)
	h.ok(&fake{}, "config", "set", "session.idle", "10m")
	r := h.ok(&fake{}, "config", "get", "session.idle")
	if strings.TrimSpace(r.out) != "10m0s" || r.err != "" {
		t.Fatalf("out %q err %q", r.out, r.err)
	}
}

func TestBadConfigStillBlocksVaultCommands(t *testing.T) {
	t.Parallel()
	h := seeded(t)
	h.writeConfig(brokenConfig)
	r := h.fail(&fake{}, "list")
	if !strings.Contains(r.err, "session.idle") {
		t.Fatalf("err %q", r.err)
	}
	if strings.Contains(r.err, "using default settings") {
		t.Fatalf("list ran on defaults: %q", r.err)
	}
}

func TestLongDurationsWarnAndAbsurdOnesAreRefused(t *testing.T) {
	t.Parallel()
	h := seeded(t)
	h.writeConfig("[session]\nidle = \"2h\"\nhard = \"20h\"\n")
	r := h.ok(&fake{}, "paths")
	if !strings.Contains(r.err, "session.hard is 20h0m0s") || !strings.Contains(r.err, "session.idle is 2h0m0s") {
		t.Fatalf("warned %q", r.err)
	}
	r = h.run(&fake{}, "config", "set", "session.hard", "48h")
	if r.code == 0 || !strings.Contains(r.err, "at most 24h0m0s") {
		t.Fatalf("err %q", r.err)
	}
}
