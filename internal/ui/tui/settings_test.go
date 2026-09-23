package tui

import (
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/elliot40404/creds/internal/app"
	"github.com/elliot40404/creds/internal/config"
	"github.com/elliot40404/creds/internal/render"
)

type fakeConfig struct {
	cfg      config.Config
	set      []string
	paths    app.PathReport
	remote   string
	trusted  []string
	probed   []string
	probeErr error
	previews int
	block    chan struct{}
	due      bool
}

func (f *fakeConfig) wait() {
	if f.block != nil {
		<-f.block
	}
}

func newFakeConfig() *fakeConfig {
	return &fakeConfig{cfg: config.Default(), remote: "git@example.test:you/vault.git", trusted: []string{"/code/app/.creds.toml"}, paths: app.PathReport{Paths: []app.PathInfo{
		{Name: "home", Path: "/v/home", Exists: true},
		{Name: "vault", Path: "/v/home/vault", Exists: true, Note: "remote https://example.test/v.git"},
		{Name: "session", Path: "/v/home/session"},
	}}}
}

func (f *fakeConfig) PathReport() app.PathReport { return f.paths }

func (f *fakeConfig) RemoteURL() string { return f.remote }

func (f *fakeConfig) CheckRemote(url string) error {
	f.wait()
	f.probed = append(f.probed, url)
	return f.probeErr
}

func (f *fakeConfig) SetRemote(url string) error {
	f.wait()
	if !strings.Contains(url, ":") {
		return errors.New("invalid remote url")
	}
	f.remote = url
	return nil
}

func (f *fakeConfig) RemoteRemove() error {
	f.wait()
	f.remote = ""
	return nil
}

func (f *fakeConfig) TrustList() ([]string, error) { return slices.Clone(f.trusted), nil }

func (f *fakeConfig) Untrust(file string) error {
	i := slices.Index(f.trusted, file)
	if i < 0 {
		return errors.New("project file is not trusted")
	}
	f.trusted = slices.Delete(f.trusted, i, i+1)
	return nil
}

func (f *fakeConfig) ConfigFields() []app.ConfigField {
	fields := config.Fields(f.cfg)
	out := make([]app.ConfigField, len(fields))
	for i, fd := range fields {
		out[i] = app.ConfigField{Key: fd.Key, Value: fd.Value(f.cfg), Default: fd.Default(), Doc: fd.Doc, Choices: fd.Choices}
		if out[i].Value == "" {
			switch fd.Key {
			case "sync.email":
				out[i].Derived, out[i].Source = "global@example.com", app.SourceGit
			case "vault.machine":
				out[i].Derived, out[i].Source = "thismachine", app.SourceHostname
			}
		}
	}
	return out
}

func (f *fakeConfig) ConfigSet(key, value string) error {
	c, err := config.Set(f.cfg, key, value)
	if err != nil {
		return err
	}
	f.cfg = c
	f.set = append(f.set, key+"="+value)
	return nil
}

func (f *fakeConfig) ConfigUnset(key string) error {
	c, err := config.Unset(f.cfg, key)
	if err != nil {
		return err
	}
	f.cfg = c
	return nil
}

func (f *fakeConfig) ConfigPreview(key, value string) (string, error) {
	f.previews++
	name, ok := strings.CutPrefix(key, config.FormatPrefix)
	if !ok {
		return "", errors.New("not a format key")
	}
	engine, format, _ := strings.Cut(name, ".")
	return render.Preview(engine, format, value, f.cfg.Render.Shell)
}

func startCfg(t *testing.T, c ConfigBackend) Model {
	t.Helper()
	m := start(t, newFake(), Hooks{})
	m.opts.Config = c
	return press(m, ch(','))
}

func TestSettingsOpensAndLists(t *testing.T) {
	m := startCfg(t, newFakeConfig())
	if _, ok := m.top().(settingsScreen); !ok {
		t.Fatalf("screen %T", m.top())
	}
	v := view(m)
	for _, key := range []string{"session.idle", "clipboard.clear"} {
		if !strings.Contains(v, key) {
			t.Fatalf("%s missing from %q", key, v)
		}
	}
	last := press(m, repeat(down, rowIndex(t, m, "render.shell"))...)
	if !strings.Contains(view(last), "render.shell") {
		t.Fatalf("render.shell not reachable: %q", view(last))
	}
}

func TestSettingsCyclesValue(t *testing.T) {
	c := newFakeConfig()
	m := startCfg(t, c)
	m = press(m, rightKey)
	if got := c.cfg.Session.Idle; got != 30*time.Minute {
		t.Fatalf("idle %v set %v", got, c.set)
	}
	if s := m.top().(settingsScreen); s.editing {
		t.Fatal("cycle row opened the text box")
	}
	v := view(m)
	if !strings.Contains(v, "(default 15m0s)") {
		t.Fatalf("default not shown %q", v)
	}
	if !strings.Contains(v, "‹ 30m0s ›") {
		t.Fatalf("markers missing %q", v)
	}
	if strings.Contains(v, "session.idle saved") {
		t.Fatalf("noisy note %q", v)
	}
	press(m, leftKey)
	if c.cfg.Session.Idle != 15*time.Minute {
		t.Fatalf("idle %v", c.cfg.Session.Idle)
	}
}

func TestSettingsCycleWraps(t *testing.T) {
	c := newFakeConfig()
	m := gotoRow(t, startCfg(t, c), "ui.mode")
	m = press(m, rightKey)
	if c.cfg.UI.Mode != config.ModeInline {
		t.Fatalf("mode %q", c.cfg.UI.Mode)
	}
	press(m, rightKey)
	if c.cfg.UI.Mode != config.ModeFullscreen {
		t.Fatalf("mode %q after wrap", c.cfg.UI.Mode)
	}
}

func TestSettingsCycleSnapsOffListValue(t *testing.T) {
	c := newFakeConfig()
	c.cfg.Session.Idle = 7 * time.Minute
	m := startCfg(t, c)
	press(m, rightKey)
	if c.cfg.Session.Idle != 15*time.Minute {
		t.Fatalf("idle %v", c.cfg.Session.Idle)
	}
}

func TestSettingsCycleKeepsInvalidValue(t *testing.T) {
	c := newFakeConfig()
	c.cfg.Session.Hard = time.Hour
	c.cfg.Session.Idle = time.Hour
	m := startCfg(t, c)
	m = press(m, rightKey)
	if c.cfg.Session.Idle != time.Hour {
		t.Fatalf("idle %v", c.cfg.Session.Idle)
	}
	if !m.failed {
		t.Fatalf("no error shown %q", view(m))
	}
}

func TestSettingsEnterStillTypesFreeText(t *testing.T) {
	m := gotoRow(t, startCfg(t, newFakeConfig()), remoteKey)
	m = press(m, enter)
	if s := m.top().(settingsScreen); !s.editing {
		t.Fatal("remote row did not open the text box")
	}
}

func TestSettingsRejectsBadValue(t *testing.T) {
	c := newFakeConfig()
	m := gotoRow(t, startCfg(t, c), remoteKey)
	m = press(m, enter)
	m = press(m, ctrl('u'))
	m = press(m, text("nope")...)
	m = press(m, enter)
	m = press(m, ch('y'))
	if c.remote != "git@example.test:you/vault.git" {
		t.Fatalf("remote %q", c.remote)
	}
	if !m.failed {
		t.Fatalf("no error shown %q", view(m))
	}
}

func TestSettingsResetToDefault(t *testing.T) {
	c := newFakeConfig()
	c.cfg.Clipboard.Clear = 5 * time.Minute
	m := startCfg(t, c)
	m = press(m, down, down, ch('r'))
	if c.cfg.Clipboard.Clear != 5*time.Minute {
		t.Fatal("reset without a confirmation")
	}
	press(m, ch('y'))
	if c.cfg.Clipboard.Clear != config.Default().Clipboard.Clear {
		t.Fatalf("clear %v", c.cfg.Clipboard.Clear)
	}
}

func TestSettingsAddsFormatWithPreview(t *testing.T) {
	c := newFakeConfig()
	m := startCfg(t, c)
	m = press(m, ch('a'))
	m = press(m, text("postgres.short")...)
	m = press(m, enter)
	m = press(m, text("psql {{sh .url}}")...)
	if !strings.Contains(view(m), "sample") || !strings.Contains(view(m), "db.example.com") {
		t.Fatalf("no preview %q", view(m))
	}
	m = press(m, enter)
	if c.cfg.Render.Formats["postgres.short"] != "psql {{sh .url}}" {
		t.Fatalf("formats %v", c.cfg.Render.Formats)
	}
	if !strings.Contains(view(m), "render.formats.postgres.short") {
		t.Fatalf("view %q", view(m))
	}
}

func TestSettingsRemovesOverrideOnly(t *testing.T) {
	c := newFakeConfig()
	c.cfg.Render.Formats["postgres.short"] = "psql {{sh .url}}"
	m := startCfg(t, c)
	m = press(m, ch('x'))
	if len(c.cfg.Render.Formats) != 1 {
		t.Fatalf("static key removed %v", c.cfg.Render.Formats)
	}
	n := rowIndex(t, m, "render.formats.postgres.short")
	press(m, append(repeat(down, n), ch('x'), ch('y'))...)
	if len(c.cfg.Render.Formats) != 0 {
		t.Fatalf("override kept %v", c.cfg.Render.Formats)
	}
}

func TestSettingsWithoutBackend(t *testing.T) {
	f := newFake()
	m := New(Options{Backend: f, Copy: f.copy, VaultPath: "/v"})
	m = step(m, tea.WindowSizeMsg{Width: 120, Height: 30})
	m = run(m, m.Init())
	m = press(m, ch(','))
	if _, ok := m.top().(settingsScreen); ok {
		t.Fatal("settings opened without a backend")
	}
	if !m.failed {
		t.Fatalf("no error %q", view(m))
	}
}

func TestSettingsEscapeLeavesValueAlone(t *testing.T) {
	c := newFakeConfig()
	m := gotoRow(t, startCfg(t, c), remoteKey)
	m = press(m, enter)
	m = press(m, text("9")...)
	m = press(m, esc)
	if c.remote != "git@example.test:you/vault.git" {
		t.Fatalf("remote %q", c.remote)
	}
	m = press(m, esc)
	if _, ok := m.top().(settingsScreen); ok {
		t.Fatal("still on settings")
	}
}

func TestSettingsShowsPaths(t *testing.T) {
	c := newFakeConfig()
	c.paths.HomeEnv = true
	v := view(startCfg(t, c))
	for _, want := range []string{"home", "/v/home/vault", "remote https://example.test/v.git", "missing", config.HomeEnv + " is set"} {
		if !strings.Contains(v, want) {
			t.Fatalf("%q missing from %q", want, v)
		}
	}
}

func TestSettingsFitsSmallScreen(t *testing.T) {
	for _, tc := range []struct {
		h     int
		files bool
	}{{12, false}, {24, true}} {
		c := newFakeConfig()
		for _, n := range []string{"config", "state", "trust", "sync.lock"} {
			c.paths.Paths = append(c.paths.Paths, app.PathInfo{Name: n, Path: "/v/home/" + n, Exists: true})
		}
		now := testNow
		f := newFake()
		m := New(Options{Backend: f, Config: c, Copy: f.copy, VaultPath: "/v", Now: func() time.Time { return now }})
		m = step(m, tea.WindowSizeMsg{Width: 80, Height: tc.h})
		m = run(m, m.Init())
		m = press(m, ch(','))
		fitsScreen(t, m.View().Content, 80, tc.h)
		if got := strings.Contains(view(m), "/v/home/sync.lock"); got != tc.files {
			t.Fatalf("height %d files shown %v, want %v", tc.h, got, tc.files)
		}
		s := m.top().(settingsScreen)
		for i := range s.rows {
			r := m.top().(settingsScreen).current()
			if v := view(m); !strings.Contains(v, r.key) || !strings.Contains(v, r.doc[:20]) {
				t.Fatalf("height %d row %d %s not shown in %q", tc.h, i, r.key, v)
			}
			m = press(m, down)
		}
		fitsScreen(t, m.View().Content, 80, tc.h)
	}
}

func gotoRow(t *testing.T, m Model, key string) Model {
	t.Helper()
	for range len(m.top().(settingsScreen).rows) {
		if m.top().(settingsScreen).current().key == key {
			return m
		}
		m = press(m, down)
	}
	t.Fatalf("no row %s", key)
	return m
}

func TestSettingsChangesRemote(t *testing.T) {
	c := newFakeConfig()
	m := gotoRow(t, startCfg(t, c), remoteKey)
	m = press(m, enter)
	m = press(m, ctrl('u'))
	m = press(m, text("git@example.test:you/other.git")...)
	m = press(m, enter)
	m = press(m, ch('y'))
	if !slices.Contains(c.probed, "git@example.test:you/other.git") {
		t.Fatalf("probed %v", c.probed)
	}
	if c.remote != "git@example.test:you/other.git" {
		t.Fatalf("remote %q", c.remote)
	}
	if v := view(m); !strings.Contains(v, "remote saved") {
		t.Fatalf("view %q", v)
	}
}

func TestSettingsRejectsBadRemote(t *testing.T) {
	c := newFakeConfig()
	m := gotoRow(t, startCfg(t, c), remoteKey)
	m = press(m, enter)
	m = press(m, ctrl('u'))
	m = press(m, text("nope")...)
	m = press(m, enter)
	m = press(m, ch('y'))
	if c.remote != "git@example.test:you/vault.git" {
		t.Fatalf("remote %q", c.remote)
	}
	if v := view(m); !strings.Contains(v, "invalid remote url") || !strings.Contains(v, "git url") {
		t.Fatalf("view %q", v)
	}
}

func TestSettingsRemovesRemote(t *testing.T) {
	c := newFakeConfig()
	m := gotoRow(t, startCfg(t, c), remoteKey)
	m = press(m, ch('x'))
	if c.remote == "" {
		t.Fatal("remote removed without a confirmation")
	}
	m = press(m, ch('y'))
	if c.remote != "" {
		t.Fatalf("remote %q", c.remote)
	}
	v := view(m)
	if !strings.Contains(v, "remote removed") || !strings.Contains(v, noneValue) {
		t.Fatalf("view %q", v)
	}
}

func TestSettingsForgetsTrustedFile(t *testing.T) {
	c := newFakeConfig()
	m := gotoRow(t, startCfg(t, c), trustKey)
	if v := view(m); !strings.Contains(v, "/code/app/.creds.toml") {
		t.Fatalf("view %q", v)
	}
	m = press(m, ch('x'))
	if len(c.trusted) != 1 {
		t.Fatalf("forgotten without a confirmation %v", c.trusted)
	}
	m = press(m, ch('y'))
	if len(c.trusted) != 0 {
		t.Fatalf("trusted %v", c.trusted)
	}
	if v := view(m); !strings.Contains(v, "forgot /code/app/.creds.toml") {
		t.Fatalf("view %q", v)
	}
}

func TestSettingsTrustedFileIsNotEditable(t *testing.T) {
	m := gotoRow(t, startCfg(t, newFakeConfig()), trustKey)
	m = press(m, enter)
	if s := m.top().(settingsScreen); s.editing {
		t.Fatal("trust row went into edit mode")
	}
}

func TestSettingsRemoteResaveKeepsLogin(t *testing.T) {
	url := remoteWithLogin()
	c := newFakeConfig()
	c.remote = url
	m := gotoRow(t, startCfg(t, c), remoteKey)
	m = press(m, enter)
	if v := view(m); strings.Contains(v, "ghp_TOKEN1234") {
		t.Fatalf("login on screen %q", v)
	}
	if v := view(press(m, enter)); !strings.Contains(v, "github.com/me/vault.git") {
		t.Fatalf("view %q", v)
	}
	if c.remote != url {
		t.Fatalf("remote %q", c.remote)
	}
}

func TestSettingsRemoteMaskedValueSavesRealURL(t *testing.T) {
	url := remoteWithLogin()
	c := newFakeConfig()
	c.remote = url
	m := gotoRow(t, startCfg(t, c), remoteKey)
	m = press(m, enter)
	m = press(m, text("https://github.com/me/vault.git")...)
	m = press(m, enter)
	if v := view(press(m, ch('y'))); !strings.Contains(v, "remote saved") {
		t.Fatalf("view %q", v)
	}
	if c.remote != url {
		t.Fatalf("remote %q", c.remote)
	}
}

func remoteWithLogin() string {
	login := "me:ghp_TOKEN1234"
	return "https://" + login + "@github.com/me/vault.git"
}

func TestSettingsRemoteProbeRefusesOtherVault(t *testing.T) {
	c := newFakeConfig()
	c.probeErr = app.ErrRemoteHasVault
	m := gotoRow(t, startCfg(t, c), remoteKey)
	m = press(m, enter)
	m = press(m, text("git@example.test:you/other.git")...)
	m = press(m, enter)
	if c.remote != "git@example.test:you/vault.git" {
		t.Fatalf("remote %q", c.remote)
	}
	if v := view(m); !strings.Contains(v, "already holds a creds vault") {
		t.Fatalf("view %q", v)
	}
}

func TestSettingsRemoteConfirmCanBeDeclined(t *testing.T) {
	c := newFakeConfig()
	m := gotoRow(t, startCfg(t, c), remoteKey)
	m = press(m, enter)
	m = press(m, text("git@example.test:you/other.git")...)
	m = press(m, enter)
	if v := view(m); !strings.Contains(v, "Push the whole vault") {
		t.Fatalf("view %q", v)
	}
	m = press(m, ch('n'))
	if c.remote != "git@example.test:you/vault.git" {
		t.Fatalf("remote %q", c.remote)
	}
	if _, ok := m.top().(settingsScreen); !ok {
		t.Fatal("not back on settings")
	}
}

func TestSettingsPreviewRendersOncePerChange(t *testing.T) {
	c := newFakeConfig()
	m := startCfg(t, c)
	m = press(m, ch('a'))
	m = press(m, text("postgres.short")...)
	m = press(m, enter)
	m = press(m, text("psql {{sh .url}}")...)
	runs := c.previews
	for range 5 {
		if !strings.Contains(view(m), "db.example.com") {
			t.Fatalf("no preview %q", view(m))
		}
	}
	if c.previews != runs {
		t.Fatalf("preview ran %d times while only drawing", c.previews-runs)
	}
}

func TestRenderPreviewRefusesHugeOutput(t *testing.T) {
	c := newFakeConfig()
	m := startCfg(t, c)
	m = press(m, ch('a'))
	m = press(m, text("postgres.big")...)
	m = press(m, enter)
	m = press(m, text(`{{define "x"}}`+strings.Repeat("A", 200)+`{{template "x"}}{{end}}{{template "x"}}`)...)
	if v := view(m); !strings.Contains(v, "too big") && !strings.Contains(v, "exceeded") {
		t.Fatalf("view %q", v)
	}
}

func rowIndex(t *testing.T, m Model, key string) int {
	t.Helper()
	s, ok := m.top().(settingsScreen)
	if !ok {
		t.Fatalf("not on settings: %#v", m.top())
	}
	for i, r := range s.rows {
		if r.key == key {
			return i - s.cursor
		}
	}
	t.Fatalf("no row %q", key)
	return 0
}

func (c *fakeConfig) SyncAfter() time.Duration {
	return 10 * time.Millisecond
}

func (c *fakeConfig) SyncDue() bool { return c.due }

func TestSettingsShowsDetectedValues(t *testing.T) {
	c := newFakeConfig()
	m := startCfg(t, c)
	v := view(press(m, repeat(down, rowIndex(t, m, "sync.email"))...))
	if !strings.Contains(v, "global@example.com  (git global)") {
		t.Fatalf("detected email not shown: %q", v)
	}
	v = view(press(m, repeat(down, rowIndex(t, m, "vault.machine"))...))
	if !strings.Contains(v, "thismachine  (hostname)") {
		t.Fatalf("detected machine not shown: %q", v)
	}
}

func TestSettingsHidesTheSourceOnceSet(t *testing.T) {
	c := newFakeConfig()
	c.cfg.Sync.Email = "mine@example.com"
	m := startCfg(t, c)
	v := view(press(m, repeat(down, rowIndex(t, m, "sync.email"))...))
	if !strings.Contains(v, "mine@example.com") {
		t.Fatalf("set value missing: %q", v)
	}
	if strings.Contains(v, "git global") || strings.Contains(v, "global@example.com") {
		t.Fatalf("source note kept after the value was set: %q", v)
	}
}

func TestSettingsRemoteInputIsMasked(t *testing.T) {
	m := gotoRow(t, startCfg(t, newFakeConfig()), remoteKey)
	m = press(m, enter, ctrl('u'))
	m = press(m, text("https://u:tok3n@example.test/v.git")...)
	if v := view(m); strings.Contains(v, "tok3n") {
		t.Fatalf("token on screen %q", v)
	}
}
