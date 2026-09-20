package cli

import (
	"slices"
	"strings"
	"testing"
)

func TestInteractiveAddEnvDefaultsSecret(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.init()
	f := &fake{
		inputs:    []string{"proj/env", "1BAD", "API_KEY", "", "MODE", "n", "dev", ""},
		picks:     []string{"env"},
		passwords: []string{envHidden},
	}
	r := h.ok(f, "add", "-i")
	noLeak(t, r, envHidden)
	got := strings.Join(printedCommand(t, r.err), " ")
	if got != "add proj/env --field MODE=dev --secret-field API_KEY --type env" {
		t.Fatalf("command %q", got)
	}
	if !slices.Contains(f.prompts, "Variable name (blank to finish)") || !slices.Contains(f.prompts, "Secret? [Y/n]") {
		t.Fatalf("prompts %q", f.prompts)
	}
	if len(f.warned) != 1 || !strings.Contains(f.warned[0], "1BAD") {
		t.Fatalf("warned %q", f.warned)
	}
}

func TestInteractiveSecretAnswerRetries(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.init()
	f := &fake{inputs: []string{"proj/env", "TOKEN", "maybe", "no", "plain", ""}, picks: []string{"env"}}
	r := h.ok(f, "add", "-i")
	if got := strings.Join(printedCommand(t, r.err), " "); got != "add proj/env --field TOKEN=plain --type env" {
		t.Fatalf("command %q", got)
	}
	if len(f.warned) != 1 {
		t.Fatalf("warned %q", f.warned)
	}
}
