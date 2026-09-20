package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func FuzzLoad(f *testing.F) {
	for _, s := range []string{
		"",
		"[session]\nidle = \"10m\"\nhard = \"1h\"",
		"[session]\nidle = \"2h\"\nhard = \"1h\"",
		"[session]\nidle = \"-1s\"",
		"[session]\nidle = 5",
		"[session]\nidle = \"9999999999h\"",
		"[clipboard]\nclear = \"0s\"",
		"[render]\nshell = \"cmd\"",
		"[render]\nshell = \"bash\"\n[render.formats]\n\"postgres.x\" = \"{{.host}}\"",
		"[render.formats]\n\"postgres.x\" = \"{{\"",
		"[render.formats]\n\"nope.x\" = \"a\"\n\"postgres\" = \"b\"",
		"[render.formats]\n\"postgres.\" = \"a\"",
		"unknown = 1",
		"[session]\nidle = \"1m\"\nidle = \"2m\"",
		"[render.formats]\n\"postgres.x\" = \"{{template \\\"postgres.x\\\"}}\"",
		"\x00",
		"a = '''\n../../=cmd'''",
	} {
		f.Add(s)
	}
	path := filepath.Join(f.TempDir(), "config.toml")
	f.Fuzz(func(t *testing.T, src string) {
		if err := os.WriteFile(path, []byte(src), 0o600); err != nil {
			t.Fatal(err)
		}
		c, err := Load(path)
		if err != nil {
			return
		}
		for _, d := range []time.Duration{c.Session.Idle, c.Session.Hard, c.Clipboard.Clear, c.Sync.Stale} {
			if d < minDuration {
				t.Fatalf("short duration accepted: %s", d)
			}
		}
		if c.Session.Idle > c.Session.Hard {
			t.Fatal("idle above hard accepted")
		}
	})
}
