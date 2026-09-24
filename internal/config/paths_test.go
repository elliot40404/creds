package config

import (
	"path/filepath"
	"testing"
)

func TestHomeEnv(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(HomeEnv, dir)
	p, err := DefaultPaths()
	got := p.Home
	if err != nil || got != dir {
		t.Fatalf("got %q %v", got, err)
	}
}

func TestHomeEnvRelative(t *testing.T) {
	t.Setenv(HomeEnv, "rel")
	p, err := DefaultPaths()
	got := p.Home
	if err != nil || !filepath.IsAbs(got) || filepath.Base(got) != "rel" {
		t.Fatalf("got %q %v", got, err)
	}
}

func TestHomeDefault(t *testing.T) {
	user := t.TempDir()
	t.Setenv(HomeEnv, "")
	t.Setenv("HOME", user)
	t.Setenv("USERPROFILE", user)
	p, err := DefaultPaths()
	got := p.Home
	want := filepath.Join(user, ".config", "creds")
	if err != nil || got != want {
		t.Fatalf("got %q %v, want %q", got, err, want)
	}
}

func TestPaths(t *testing.T) {
	h := filepath.Join("x", "creds")
	p := Paths{Home: h}
	cases := map[string]string{
		p.Vault():    "vault",
		p.Session():  "session",
		p.State():    "state.json",
		p.SyncLock(): "sync.lock",
		p.Config():   "config.toml",
		p.Device():   "device.json",
	}
	for got, name := range cases {
		if want := filepath.Join(h, name); got != want {
			t.Fatalf("got %q, want %q", got, want)
		}
	}
}

func TestDefaultPaths(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(HomeEnv, dir)
	p, err := DefaultPaths()
	if err != nil || p.Home != dir {
		t.Fatalf("got %+v %v", p, err)
	}
}
