package cli

import (
	"encoding/json"
	"os"
	"runtime"
	"strings"
	"testing"

	"github.com/elliot40404/creds/internal/config"
	"github.com/elliot40404/creds/internal/testutil"
)

func sealedMethod() string {
	if runtime.GOOS == "windows" {
		return "dpapi"
	}
	return "fake"
}

func (h *harness) session() map[string]any {
	h.t.Helper()
	data, err := os.ReadFile(config.Paths{Home: h.home}.Session())
	if err != nil {
		h.t.Fatal(err)
	}
	var r map[string]any
	if err := json.Unmarshal(data, &r); err != nil {
		h.t.Fatal(err)
	}
	return r
}

func (h *harness) unlockAgain() result {
	h.t.Helper()
	h.ok(&fake{}, "lock")
	return h.ok(&fake{passwords: []string{mainWord}}, "unlock")
}

func TestUnlockSealsSessionAndCopyWorks(t *testing.T) {
	h := seeded(t)
	h.unlockAgain()
	r := h.session()
	if r["method"] != sealedMethod() || r["identity"] != nil || r["sealed"] == nil {
		t.Fatalf("session %v", r)
	}
	h.ok(&fake{}, "copy", "web/mail")
	if h.clip.value != loginHidden {
		t.Fatalf("clip %q", h.clip.value)
	}
	if out := h.ok(&fake{}, "paths").out; !strings.Contains(out, sealedMethod()+", ") {
		t.Fatalf("paths %q", out)
	}
}

func TestProtectOffWritesPlainSession(t *testing.T) {
	h := seeded(t)
	h.ok(&fake{}, "config", "set", "session.protect", "false")
	h.unlockAgain()
	if r := h.session(); r["method"] != "plain" || r["identity"] == nil {
		t.Fatalf("session %v", r)
	}
	if out := h.ok(&fake{}, "paths").out; !strings.Contains(out, "plain, ") {
		t.Fatalf("paths %q", out)
	}
}

func TestNoKeystoreWarnsOnUnlock(t *testing.T) {
	h := seeded(t)
	h.keys = testutil.NoKeystore
	r := h.unlockAgain()
	if !strings.Contains(r.err, "saved as a plain file, run creds lock") {
		t.Fatalf("stderr %q", r.err)
	}
	if s := h.session(); s["method"] != "plain" {
		t.Fatalf("session %v", s)
	}
	if r := h.ok(&fake{}, "copy", "web/mail"); strings.Contains(r.err, "plain file") {
		t.Fatalf("warned again %q", r.err)
	}
}
