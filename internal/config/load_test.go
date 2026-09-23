package config

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/elliot40404/creds/internal/render"
)

func writeConfig(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadMissing(t *testing.T) {
	c, err := Load(filepath.Join(t.TempDir(), "config.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(c, Default()) {
		t.Fatalf("got %+v", c)
	}
}

func TestLoadEmpty(t *testing.T) {
	c, err := Load(writeConfig(t, ""))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(c, Default()) {
		t.Fatalf("got %+v", c)
	}
}

func TestLoadOverride(t *testing.T) {
	c, err := Load(writeConfig(t, `
[session]
idle = "5m"

[sync]
stale = "1h"

[render]
shell = "pwsh"

[render.formats]
"postgres.pg" = "postgres://{{.username}}@{{.host}}"
`))
	if err != nil {
		t.Fatal(err)
	}
	want := Default()
	want.Session.Idle = 5 * time.Minute
	want.Sync.Stale = time.Hour
	want.Render.Shell = render.Pwsh
	want.Render.Formats = map[string]string{"postgres.pg": "postgres://{{.username}}@{{.host}}"}
	if !reflect.DeepEqual(c, want) {
		t.Fatalf("got %+v, want %+v", c, want)
	}
}

func TestLoadBad(t *testing.T) {
	cases := map[string]struct{ body, msg string }{
		"syntax":            {`[session`, ""},
		"unknown key":       {"[session]\nidel = \"5m\"", "session.idel"},
		"unknown table":     {"[colors]\nx = 1", "colors"},
		"unknown top key":   {`foo = "bar"`, "foo"},
		"bad duration":      {"[session]\nidle = \"soon\"", ""},
		"int duration":      {"[session]\nidle = 15", "session.idle"},
		"zero duration":     {"[clipboard]\nclear = \"0s\"", "clipboard.clear"},
		"negative duration": {"[sync]\nstale = \"-5m\"", "sync.stale"},
		"idle over hard":    {"[session]\nidle = \"5h\"", "session.idle"},
		"wrong type":        {"[session]\nidle = true", ""},
		"unknown shell":     {"[render]\nshell = \"fish\"", "render.shell"},
		"format not string": {"[render.formats]\npg = 1", ""},
		"format name ctrl":  {"[render.formats]\n\"postgres.x\\u001b]0;x\\u0007\" = \"x\"", "render.formats"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			path := writeConfig(t, tc.body)
			_, err := Load(path)
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), path) || !strings.Contains(err.Error(), tc.msg) {
				t.Fatalf("error %q missing %q", err, tc.msg)
			}
		})
	}
}

func TestLoadReadError(t *testing.T) {
	if _, err := Load(t.TempDir()); err == nil {
		t.Fatal("expected error reading directory")
	}
}

func TestLoadBadNamesTheGivenPath(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "dotfiles.toml")
	if err := os.WriteFile(target, []byte(`[session`), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "config.toml")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}
	_, err := Load(link)
	if err == nil || !strings.Contains(err.Error(), link) {
		t.Fatalf("error %v does not name %s", err, link)
	}
}
