package cli

import (
	"strings"
	"testing"
)

func firstRow(s string) string {
	lines := strings.Split(s, "\n")
	if len(lines) < 2 {
		return ""
	}
	return lines[1]
}

func TestListPutsUsedEntriesFirst(t *testing.T) {
	t.Parallel()
	h := seeded(t)
	if r := h.ok(&fake{}, "list"); !strings.HasPrefix(firstRow(r.out), "db/prod") {
		t.Fatalf("before any use %q", r.out)
	}
	h.ok(&fake{}, "get", "web/mail")
	if r := h.ok(&fake{}, "list"); !strings.HasPrefix(firstRow(r.out), "web/mail") {
		t.Fatalf("used entry not first %q", r.out)
	}
	if r := h.ok(&fake{}, "list", "--sort", "path"); !strings.HasPrefix(firstRow(r.out), "db/prod") {
		t.Fatalf("path sort %q", r.out)
	}
	h.ok(&fake{}, "config", "set", "search.frecency", "false")
	if r := h.ok(&fake{}, "list"); !strings.HasPrefix(firstRow(r.out), "db/prod") {
		t.Fatalf("frecency off still ranks by use %q", r.out)
	}
}
