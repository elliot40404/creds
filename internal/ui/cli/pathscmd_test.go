package cli

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/elliot40404/creds/internal/app"
	"github.com/elliot40404/creds/internal/config"
)

func TestPathsListsEveryFileOnAnEmptyHome(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	r := h.ok(&fake{}, "paths")
	for _, name := range []string{"home", "config", "vault", "session", "state", "trust", "sync.lock"} {
		if !strings.Contains(r.out, name) {
			t.Fatalf("%s missing from %q", name, r.out)
		}
	}
	if strings.Count(r.out, "missing") != 6 || strings.Count(r.out, "exists") != 1 {
		t.Fatalf("out %q", r.out)
	}
	if !strings.Contains(r.out, h.home) {
		t.Fatalf("home path missing from %q", r.out)
	}
}

func TestPathsShowsVaultRemoteAndSession(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.init()
	remote := bareRemote(t)
	h.ok(&fake{}, "remote", "add", remote)
	r := h.ok(&fake{}, "paths")
	if !strings.Contains(r.out, "remote "+remote) {
		t.Fatalf("remote missing from %q", r.out)
	}
	if !strings.Contains(r.out, "left") {
		t.Fatalf("session time left missing from %q", r.out)
	}
	h.expire()
	r = h.ok(&fake{}, "paths")
	if !strings.Contains(r.out, "expired") {
		t.Fatalf("out %q", r.out)
	}
}

func TestPathsJSON(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	r := h.ok(&fake{}, "paths", "--json")
	var got app.PathReport
	if err := json.Unmarshal([]byte(r.out), &got); err != nil {
		t.Fatalf("%v: %q", err, r.out)
	}
	if len(got.Paths) != 7 || got.Paths[0].Name != "home" || got.Paths[0].Path != h.home {
		t.Fatalf("report %+v", got)
	}
	for _, p := range got.Paths[1:] {
		if p.Exists {
			t.Fatalf("%s should not exist", p.Name)
		}
	}
}

func TestPathsNotesCredsHome(t *testing.T) {
	h := newHarness(t)
	t.Setenv(config.HomeEnv, h.home)
	r := h.ok(&fake{}, "paths")
	if !strings.Contains(r.err, config.HomeEnv+" is set") {
		t.Fatalf("stderr %q", r.err)
	}
}
