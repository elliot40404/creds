package config

import (
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/elliot40404/creds/internal/render"
)

func TestKeysCoverEveryValue(t *testing.T) {
	t.Parallel()
	keys := fieldKeys(Default())
	for _, want := range []string{"session.idle", "session.hard", "clipboard.clear", "sync.stale", "render.shell"} {
		if !slices.Contains(keys, want) {
			t.Fatalf("%s missing from %v", want, keys)
		}
	}
}

func TestGetReturnsCurrentValue(t *testing.T) {
	t.Parallel()
	c := Default()
	c.Session.Idle = 90 * time.Second
	got, err := Get(c, "session.idle")
	if err != nil || got != "1m30s" {
		t.Fatalf("got %q %v", got, err)
	}
}

func TestGetUnknownKey(t *testing.T) {
	t.Parallel()
	if _, err := Get(Default(), "session.nope"); !errors.Is(err, ErrUnknownKey) {
		t.Fatalf("err %v", err)
	}
}

func TestSetDuration(t *testing.T) {
	t.Parallel()
	c, err := Set(Default(), "clipboard.clear", "45s")
	if err != nil {
		t.Fatal(err)
	}
	if c.Clipboard.Clear != 45*time.Second {
		t.Fatalf("clear %v", c.Clipboard.Clear)
	}
}

func TestSetRejectsBadValues(t *testing.T) {
	t.Parallel()
	cases := []struct{ key, value string }{
		{"session.idle", "nope"},
		{"session.idle", "500ms"},
		{"session.idle", "99h"},
		{"render.shell", "fish"},
		{"render.formats.postgres.bad", "{{ .Nope "},
	}
	for _, tc := range cases {
		if _, err := Set(Default(), tc.key, tc.value); err == nil {
			t.Fatalf("%s=%s accepted", tc.key, tc.value)
		}
	}
}

func TestSetLeavesOriginalAlone(t *testing.T) {
	t.Parallel()
	c := Default()
	c.Render.Formats["postgres.one"] = "{{ .url }}"
	if _, err := Set(c, "render.formats.postgres.two", "{{ .url }}"); err != nil {
		t.Fatal(err)
	}
	if _, ok := c.Render.Formats["postgres.two"]; ok {
		t.Fatal("source config changed")
	}
}

func TestFormatFieldRoundTrip(t *testing.T) {
	t.Parallel()
	c, err := Set(Default(), "render.formats.postgres.env", "{{ .url }}")
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := Get(c, "render.formats.postgres.env"); got != "{{ .url }}" {
		t.Fatalf("got %q", got)
	}
	if !slices.Contains(fieldKeys(c), "render.formats.postgres.env") {
		t.Fatalf("keys %v", fieldKeys(c))
	}
	c, err = Unset(c, "render.formats.postgres.env")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := c.Render.Formats["postgres.env"]; ok {
		t.Fatal("still set")
	}
}

func TestUnsetStaticKeyRefused(t *testing.T) {
	t.Parallel()
	if _, err := Unset(Default(), "session.idle"); !errors.Is(err, ErrNotRemovable) {
		t.Fatalf("err %v", err)
	}
}

func TestFieldDefault(t *testing.T) {
	t.Parallel()
	c := Default()
	c.Session.Hard = 9 * time.Hour
	for _, f := range Fields(c) {
		if f.Key != "session.hard" {
			continue
		}
		if f.Value(c) != "9h0m0s" || f.Default() != "4h0m0s" {
			t.Fatalf("value %q default %q", f.Value(c), f.Default())
		}
		if f.Doc == "" {
			t.Fatal("no doc")
		}
	}
}

func TestNextWrapsBothWays(t *testing.T) {
	t.Parallel()
	got, err := Next("ui.mode", string(ModeInline), 1)
	if err != nil || got != string(ModeFullscreen) {
		t.Fatalf("got %q %v", got, err)
	}
	if got, err := Next("ui.mode", string(ModeFullscreen), -1); err != nil || got != string(ModeInline) {
		t.Fatalf("got %q %v", got, err)
	}
	if got, err := Next("session.idle", "1m0s", -1); err != nil || got != "4h0m0s" {
		t.Fatalf("got %q %v", got, err)
	}
}

func TestNextSnapsOffListValue(t *testing.T) {
	t.Parallel()
	if got, err := Next("session.idle", "7m", 1); err != nil || got != "15m0s" {
		t.Fatalf("up got %q %v", got, err)
	}
	if got, err := Next("session.idle", "7m", -1); err != nil || got != "5m0s" {
		t.Fatalf("down got %q %v", got, err)
	}
	if got, err := Next("ui.height", "22", 1); err != nil || got != "25" {
		t.Fatalf("up got %q %v", got, err)
	}
	if got, err := Next("ui.height", "22", -1); err != nil || got != "20" {
		t.Fatalf("down got %q %v", got, err)
	}
}

func TestNextEmptyShellTakesFirstChoice(t *testing.T) {
	t.Parallel()
	got, err := Next("render.shell", "", 1)
	if err != nil || got != render.Shells()[0] {
		t.Fatalf("got %q %v", got, err)
	}
}

func TestNextWithoutChoices(t *testing.T) {
	t.Parallel()
	if _, err := Next(FormatPrefix+"postgres.short", "x", 1); !errors.Is(err, ErrNoChoices) {
		t.Fatalf("err %v", err)
	}
	if _, err := Next("session.nope", "x", 1); !errors.Is(err, ErrUnknownKey) {
		t.Fatalf("err %v", err)
	}
}

func TestEveryChoiceIsValid(t *testing.T) {
	t.Parallel()
	for _, f := range Fields(Default()) {
		for _, v := range f.Choices {
			if _, err := Set(Default(), f.Key, v); err != nil {
				t.Errorf("%s = %s: %v", f.Key, v, err)
			}
		}
		if len(f.Choices) > 0 && !slices.Contains(f.Choices, f.Default()) {
			t.Errorf("%s default %q is not a choice %v", f.Key, f.Default(), f.Choices)
		}
	}
}

func fieldKeys(c Config) []string {
	var keys []string
	for _, f := range Fields(c) {
		keys = append(keys, f.Key)
	}
	return keys
}

func TestDeviceMaxAgeKey(t *testing.T) {
	t.Parallel()
	if v, err := Get(Default(), "device.max_age"); err != nil || v != "72h0m0s" {
		t.Fatalf("get %q %v", v, err)
	}
	c, err := Set(Default(), "device.max_age", "24h")
	if err != nil || c.Device.MaxAge != 24*time.Hour {
		t.Fatalf("set %+v %v", c.Device, err)
	}
	if _, err := Unset(c, "device.max_age"); !errors.Is(err, ErrNotRemovable) {
		t.Fatalf("unset %v", err)
	}
	data, err := Encode(c)
	if err != nil {
		t.Fatal(err)
	}
	back, err := Decode(data)
	if err != nil || back.Device.MaxAge != 24*time.Hour {
		t.Fatalf("roundtrip %+v %v", back.Device, err)
	}
}
