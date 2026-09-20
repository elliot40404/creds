package config

import (
	"maps"
	"path/filepath"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/elliot40404/creds/internal/render"
)

func FuzzSave(f *testing.F) {
	f.Add(int64(time.Minute), int64(time.Hour), int64(time.Second), int64(time.Minute), "bash", "postgres.psql", "psql {{.url}}")
	f.Add(int64(0), int64(0), int64(0), int64(0), "", "", "")
	f.Add(int64(-1), int64(1), int64(1), int64(1), "cmd", "postgres.x", "{{")
	f.Add(int64(time.Second), int64(time.Second), int64(time.Second), int64(time.Second), "pwsh", "postgres.x", "a\nb\"'#=")
	f.Add(int64(time.Second), int64(time.Second), int64(time.Second), int64(time.Second), "pwsh", "postgres.x", "\x00 ")
	f.Add(int64(1<<62), int64(1<<62), int64(1<<62), int64(1<<62), "bash", "redis.x", "{{.host}}")
	f.Fuzz(func(t *testing.T, idle, hard, clear, stale int64, shell, key, val string) {
		want := Config{
			Session:   Session{Idle: time.Duration(idle), Hard: time.Duration(hard)},
			Clipboard: Clipboard{Clear: time.Duration(clear)},
			Sync:      Sync{Stale: time.Duration(stale)},
			Render:    Render{Shell: render.Shell(shell), Formats: map[string]string{key: val}},
		}
		if !utf8.ValidString(shell) || !utf8.ValidString(key) || !utf8.ValidString(val) {
			return
		}
		path := filepath.Join(t.TempDir(), "config.toml")
		if err := Save(path, want); err != nil {
			return
		}
		got, err := Load(path)
		if err != nil {
			t.Fatalf("load after save: %v", err)
		}
		if got.Session != want.Session || got.Clipboard != want.Clipboard || got.Sync != want.Sync {
			t.Fatalf("durations changed: got %+v, want %+v", got, want)
		}
		if got.Render.Shell != want.Render.Shell && string(want.Render.Shell) != "" {
			t.Fatalf("shell changed: got %q, want %q", got.Render.Shell, want.Render.Shell)
		}
		if !maps.Equal(got.Render.Formats, want.Render.Formats) {
			t.Fatalf("formats changed: got %q, want %q", got.Render.Formats, want.Render.Formats)
		}
	})
}
