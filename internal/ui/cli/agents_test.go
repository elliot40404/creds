package cli

import (
	"strings"
	"testing"
)

func TestHelpAgents(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	r := h.ok(&fake{}, "help", "agents")
	for _, want := range []string{
		"creds unlock", "--json", "--secret-field", "--yes", "3 locked", "6 no vault",
		"creds paths --json", "creds config --json", "creds trust list --json",
		"creds config set", "creds trust remove", "creds device status --json",
		"creds ssh status --json", "creds ssh push --json", "creds ssh pull --json", "creds ssh add-key",
	} {
		if !strings.Contains(r.out, want) {
			t.Fatalf("missing %q", want)
		}
	}
	if r = h.ok(&fake{}, "--help"); !strings.Contains(r.out, "agents") {
		t.Fatalf("topic not listed: %q", r.out)
	}
}
