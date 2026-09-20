package cli

import (
	"strings"
	"testing"

	"github.com/elliot40404/creds/internal/config"
	"github.com/elliot40404/creds/internal/gitsync"
)

func TestEscapedSyncBanner(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.init()
	h.ok(&fake{}, "remote", "add", bareRemote(t))
	st := gitsync.State{LastSync: h.now, LastResult: "error", LastError: "git fetch: " + evil + "\nline2"}
	if err := gitsync.SaveState(config.Paths{Home: h.home}.State(), st); err != nil {
		t.Fatal(err)
	}
	r := h.ok(&fake{}, "list")
	noEscape(t, "banner", r.err)
	if !strings.HasPrefix(r.err, `last sync failed: x\x1b]52;c;cHduZWQ=\x07`) {
		t.Fatalf("banner = %q", r.err)
	}
	r = h.ok(&fake{}, "status")
	noEscape(t, "status", r.out+r.err)
	if !strings.Contains(r.out, "last error: x\\x1b") {
		t.Fatalf("status = %q", r.out)
	}
}

func TestStatusHidesRemoteCredentials(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.init()
	h.ok(&fake{}, "remote", "add", "https://bob:"+"tok3n"+"@example.com/v.git")
	r := h.ok(&fake{}, "status")
	if strings.Contains(r.out, "tok3n") || !strings.Contains(r.out, "remote: https://example.com/v.git\n") {
		t.Fatalf("status = %q", r.out)
	}
}
