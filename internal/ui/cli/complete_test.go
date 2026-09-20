package cli

import (
	"strings"
	"testing"
)

func complete(t *testing.T, h *harness, args ...string) string {
	t.Helper()
	f := &fake{}
	r := h.ok(f, append([]string{"__complete"}, args...)...)
	if len(f.prompts) != 0 {
		t.Fatalf("completion prompted: %v", f.prompts)
	}
	return r.out
}

func TestCompleteWithSession(t *testing.T) {
	t.Parallel()
	h := seeded(t)
	for _, cmd := range []string{"get", "edit", "rm"} {
		out := complete(t, h, cmd, "")
		if !strings.Contains(out, "web/mail\n") || !strings.Contains(out, "db/prod\n") {
			t.Fatalf("%s: %q", cmd, out)
		}
	}
	out := complete(t, h, "get", "db")
	if !strings.Contains(out, "db/prod") || strings.Contains(out, "web/mail") {
		t.Fatalf("prefix: %q", out)
	}
	out = complete(t, h, "get", "db/prod", "")
	if strings.Contains(out, "web/mail") {
		t.Fatalf("second arg: %q", out)
	}
}

func TestCompleteWithoutSession(t *testing.T) {
	t.Parallel()
	h := seeded(t)
	h.ok(&fake{}, "lock")
	out := complete(t, h, "get", "")
	if strings.Contains(out, "/") {
		t.Fatalf("locked: %q", out)
	}
	h.ok(&fake{passwords: []string{mainWord}}, "unlock")
	h.expire()
	out = complete(t, h, "rm", "")
	if strings.Contains(out, "/") {
		t.Fatalf("expired: %q", out)
	}
}

func TestCompleteWithoutVault(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	out := complete(t, h, "get", "")
	if strings.Contains(out, "/") {
		t.Fatalf("no vault: %q", out)
	}
}

func TestCompleteAddOffersFoldersOnly(t *testing.T) {
	t.Parallel()
	h := seeded(t)
	out := complete(t, h, "add", "")
	for _, folder := range []string{"db/\n", "web/\n"} {
		if !strings.Contains(out, folder) {
			t.Fatalf("missing %q in %q", folder, out)
		}
	}
	for _, full := range []string{"db/prod", "web/mail"} {
		if strings.Contains(out, full) {
			t.Fatalf("offered the existing path %q in %q", full, out)
		}
	}
	if !strings.Contains(out, ":6") {
		t.Fatalf("want NoFileComp|NoSpace directive, got %q", out)
	}
	out = complete(t, h, "add", "we")
	if !strings.Contains(out, "web/") || strings.Contains(out, "db/") {
		t.Fatalf("prefix: %q", out)
	}
	out = complete(t, h, "add", "new/path", "")
	if strings.Contains(out, "/") {
		t.Fatalf("second arg: %q", out)
	}
}
