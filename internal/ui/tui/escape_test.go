package tui

import (
	"errors"
	"strings"
	"testing"

	"github.com/elliot40404/creds/internal/app"
	"github.com/elliot40404/creds/internal/vault"
)

var evil = "x\x1b]52;c;cHduZWQ=\x07\x1b]8;;http://a\x07" + string(rune(0x9b)) + "31m"

func noRawEscape(t *testing.T, m Model) {
	t.Helper()
	raw := m.View().Content
	for _, bad := range []string{"\x1b]", "\x07", "\x1b[2J", string(rune(0x9b)), "\r"} {
		if strings.Contains(raw, bad) {
			t.Fatalf("view has %q: %q", bad, raw)
		}
	}
	if !strings.Contains(view(m), `\x1b]52`) {
		t.Fatalf("escaped text missing: %q", view(m))
	}
}

func TestViewEscapesUntrustedText(t *testing.T) {
	f := &fakeBackend{entries: []vault.Entry{{
		Path: "a" + evil, Type: vault.TypeLogin, Username: evil, Host: evil, Notes: evil + "\x1b[2J",
		Tags:   []string{evil},
		Fields: []vault.Field{{Name: "k" + evil, Value: evil}},
	}}}
	f.status = app.SyncStatus{Remote: "https://h/" + evil, LastError: evil}
	f.syncErr = errors.New("fetch: " + evil)
	m := start(t, f, Hooks{})
	noRawEscape(t, m)
	m = press(m, enter)
	noRawEscape(t, m)
	m = press(m, down, down, down)
	noRawEscape(t, m)
	m = press(m, ch('s'))
	noRawEscape(t, m)
	if !strings.Contains(view(m), `sync failed: fetch: x\x1b]52`) {
		t.Fatalf("view %q", view(m))
	}
}
