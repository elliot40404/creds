package config

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSaveRoundTrip(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "config.toml")
	want := Default()
	want.Session.Idle = 90 * time.Second
	want.Render.Formats = map[string]string{"postgres.psql": "psql {{.url}}"}
	if err := Save(path, want); err != nil {
		t.Fatalf("save: %v", err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got.Session != want.Session || got.Clipboard != want.Clipboard || got.Sync != want.Sync {
		t.Fatalf("got %+v, want %+v", got, want)
	}
	if got.Render.Shell != want.Render.Shell || got.Render.Formats["postgres.psql"] != want.Render.Formats["postgres.psql"] {
		t.Fatalf("render = %+v", got.Render)
	}
}

func TestSaveRejectsInvalid(t *testing.T) {
	t.Parallel()
	c := Default()
	c.Session.Idle = 0
	if err := Save(filepath.Join(t.TempDir(), "config.toml"), c); err == nil {
		t.Fatal("want error")
	}
}

func TestSaveRejectsInvalidUTF8(t *testing.T) {
	t.Parallel()
	for name, c := range map[string]Config{
		"template": withFormat("postgres.psql", "psql \xff"),
		"name":     withFormat("postgres.\xff", "psql {{.url}}"),
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			path := filepath.Join(t.TempDir(), "config.toml")
			if err := Save(path, c); err == nil {
				t.Fatal("want error")
			}
			if _, err := os.Stat(path); !errors.Is(err, fs.ErrNotExist) {
				t.Fatalf("file written: %v", err)
			}
		})
	}
}

func withFormat(name, tmpl string) Config {
	c := Default()
	c.Render.Formats = map[string]string{name: tmpl}
	return c
}

func TestSaveWritesThroughASymlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "dotfiles.toml")
	if err := os.WriteFile(target, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "config.toml")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}
	c := Default()
	c.Session.Idle = 20 * time.Minute
	if err := Save(link, c); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Lstat(link); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("link replaced: %v", err)
	}
	got, err := Load(target)
	if err != nil || got.Session.Idle != 20*time.Minute {
		t.Fatalf("target not written: %+v %v", got.Session, err)
	}
	if got, err := Load(link); err != nil || got.Session.Idle != 20*time.Minute {
		t.Fatalf("load through the link: %+v %v", got.Session, err)
	}
}
