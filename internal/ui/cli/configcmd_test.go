package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func (h *harness) edit(f *fake, fn func(string) error, args ...string) result {
	h.t.Helper()
	env, out, errb := h.env(f)
	env.Editor = fn
	code := Run(env, args)
	return result{code: code, out: out.String(), err: errb.String()}
}

func TestConfigListsKeysAndDefaults(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	r := h.ok(&fake{}, "config")
	for _, key := range []string{"session.idle", "session.hard", "clipboard.clear", "sync.stale", "render.shell"} {
		if !strings.Contains(r.out, key) {
			t.Fatalf("%s missing from %q", key, r.out)
		}
	}
	if strings.Contains(r.out, "default") {
		t.Fatalf("defaults marked on a default config: %q", r.out)
	}
	h.ok(&fake{}, "config", "set", "session.idle", "30m")
	r = h.ok(&fake{}, "config")
	if !strings.Contains(r.out, "30m0s  (default 15m0s)") {
		t.Fatalf("out %q", r.out)
	}
}

func TestConfigJSON(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	r := h.ok(&fake{}, "config", "--json")
	var fields []struct{ Key, Value, Default, Doc string }
	if err := json.Unmarshal([]byte(r.out), &fields); err != nil {
		t.Fatalf("%v: %q", err, r.out)
	}
	if len(fields) < 5 || fields[0].Doc == "" {
		t.Fatalf("fields %+v", fields)
	}
}

func TestConfigGetJSONOmitsEmptyFields(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	r := h.ok(&fake{}, "config", "get", "session.idle", "--json")
	var got map[string]any
	if err := json.Unmarshal([]byte(r.out), &got); err != nil {
		t.Fatalf("%v: %q", err, r.out)
	}
	if got["key"] != "session.idle" || got["value"] != "15m0s" {
		t.Fatalf("got %+v", got)
	}
	if _, ok := got["default"]; ok {
		t.Fatalf("default present: %q", r.out)
	}
	if _, ok := got["doc"]; ok {
		t.Fatalf("doc present: %q", r.out)
	}
}

func TestConfigGetAndSet(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.ok(&fake{}, "config", "set", "clipboard.clear", "45s")
	r := h.ok(&fake{}, "config", "get", "clipboard.clear")
	if strings.TrimSpace(r.out) != "45s" {
		t.Fatalf("out %q", r.out)
	}
	data, err := os.ReadFile(filepath.Join(h.home, "config.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "clear = \"45s\"") {
		t.Fatalf("file %q", data)
	}
}

func TestConfigSetRejectsBadValue(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	for _, args := range [][]string{
		{"config", "set", "session.idle", "nope"},
		{"config", "set", "session.idle", "10h"},
		{"config", "set", "render.shell", "fish"},
		{"config", "set", "nope.nope", "1s"},
		{"config", "get", "nope.nope"},
		{"config", "unset", "session.idle"},
	} {
		r := h.fail(&fake{}, args...)
		if r.err == "" {
			t.Fatalf("%v: no message", args)
		}
	}
	if _, err := os.Stat(filepath.Join(h.home, "config.toml")); err == nil {
		t.Fatal("bad value written")
	}
}

func TestConfigSetBadDurationSaysHow(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	r := h.fail(&fake{}, "config", "set", "session.idle", "banana")
	want := `session.idle: not a duration: "banana"`
	if !strings.Contains(r.err, want) || strings.Contains(r.err, "time:") || !strings.Contains(r.err, "like 90s or 4h") {
		t.Fatalf("err %q", r.err)
	}
}

func TestConfigFormatOverride(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.ok(&fake{}, "config", "set", "render.formats.postgres.psql", "psql {{ sh .url }}")
	r := h.ok(&fake{}, "config", "get", "render.formats.postgres.psql")
	if strings.TrimSpace(r.out) != "psql {{ sh .url }}" {
		t.Fatalf("out %q", r.out)
	}
	h.ok(&fake{}, "config", "unset", "render.formats.postgres.psql")
	h.fail(&fake{}, "config", "get", "render.formats.postgres.nope")
}

func TestConfigEditSaves(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	r := h.edit(&fake{}, func(path string) error {
		return os.WriteFile(path, []byte("[session]\nidle = \"20m\"\nhard = \"5h\"\n"), 0o600)
	}, "config", "edit")
	if r.code != 0 {
		t.Fatalf("code %d err %q", r.code, r.err)
	}
	got := h.ok(&fake{}, "config", "get", "session.idle")
	if strings.TrimSpace(got.out) != "20m0s" {
		t.Fatalf("out %q", got.out)
	}
}

func TestConfigEditRejectsBadFile(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	r := h.edit(&fake{}, func(path string) error {
		return os.WriteFile(path, []byte("[session]\nidle = \"5h\"\nhard = \"1m\"\n"), 0o600)
	}, "config", "edit")
	if r.code == 0 {
		t.Fatalf("bad file accepted: %q", r.err)
	}
	if _, err := os.Stat(filepath.Join(h.home, "config.toml")); err == nil {
		t.Fatal("bad file written")
	}
}

func TestConfigEditUnchanged(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	r := h.edit(&fake{}, func(string) error { return nil }, "config", "edit")
	if r.code != 0 || !strings.Contains(r.err, "unchanged") {
		t.Fatalf("code %d err %q", r.code, r.err)
	}
}

func TestConfigJSONCarriesChoices(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	r := h.ok(&fake{}, "config", "--json")
	var fields []struct {
		Key     string
		Choices []string
	}
	if err := json.Unmarshal([]byte(r.out), &fields); err != nil {
		t.Fatalf("%v: %q", err, r.out)
	}
	for _, f := range fields {
		if f.Key == "ui.mode" && len(f.Choices) == 2 {
			return
		}
	}
	t.Fatalf("no choices for ui.mode: %q", r.out)
}

func TestConfigSetInteractivePicksFromList(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	f := &fake{picks: []string{"ui.mode = fullscreen", "inline"}}
	h.ok(f, "config", "set", "-i")
	if !f.drained() {
		t.Fatalf("prompts left %v", f.picks)
	}
	r := h.ok(&fake{}, "config", "get", "ui.mode")
	if strings.TrimSpace(r.out) != "inline" {
		t.Fatalf("out %q", r.out)
	}
}

func TestConfigListShowsDetectedValues(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.init()
	out := h.ok(&fake{}, "config").out
	for _, key := range []string{"sync.name", "sync.email", "vault.machine"} {
		line := configLine(t, out, key)
		if strings.HasSuffix(strings.TrimSpace(line), key) {
			t.Fatalf("%s shows nothing at all: %q", key, line)
		}
		if !strings.Contains(line, "(git global)") && !strings.Contains(line, "(hostname)") && !strings.Contains(line, "(built in)") {
			t.Fatalf("%s has no source note: %q", key, line)
		}
	}
	if strings.Contains(out, "(default )") {
		t.Fatalf("empty default printed: %q", out)
	}
}

func configLine(t *testing.T, out, key string) string {
	t.Helper()
	for line := range strings.SplitSeq(out, "\n") {
		if strings.HasPrefix(line, key+" ") {
			return line
		}
	}
	t.Fatalf("no line for %q in %q", key, out)
	return ""
}
