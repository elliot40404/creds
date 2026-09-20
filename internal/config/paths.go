package config

import (
	"os"
	"path/filepath"
)

const HomeEnv = "CREDS_HOME"

type Paths struct {
	Home string
}

func DefaultPaths() (Paths, error) {
	if h := os.Getenv(HomeEnv); h != "" {
		abs, err := filepath.Abs(h)
		return Paths{Home: abs}, err
	}
	user, err := os.UserHomeDir()
	if err != nil {
		return Paths{}, err
	}
	return Paths{Home: filepath.Join(user, ".config", "creds")}, nil
}

func (p Paths) Vault() string    { return filepath.Join(p.Home, "vault") }
func (p Paths) Session() string  { return filepath.Join(p.Home, "session") }
func (p Paths) State() string    { return filepath.Join(p.Home, "state.json") }
func (p Paths) SyncLock() string { return filepath.Join(p.Home, "sync.lock") }
func (p Paths) Config() string   { return filepath.Join(p.Home, "config.toml") }
func (p Paths) Trust() string    { return filepath.Join(p.Home, "trust.json") }
func (p Paths) Anchor() string   { return filepath.Join(p.Home, "anchor.json") }
