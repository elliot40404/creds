package config

import (
	"errors"
	"time"

	"github.com/elliot40404/creds/internal/render"
	"github.com/elliot40404/creds/internal/vaultfiles"
)

var (
	ErrUnknownKey   = errors.New("unknown config key")
	ErrNotRemovable = errors.New("config key cannot be removed")
	ErrBadDuration  = errors.New("not a duration")
	ErrNotBool      = errors.New("not true or false")
	ErrNotNumber    = errors.New("not a whole number")
	ErrNoChoices    = errors.New("config key has no list of values")
)

type Config struct {
	Session   Session   `toml:"session"`
	Clipboard Clipboard `toml:"clipboard"`
	Sync      Sync      `toml:"sync"`
	Render    Render    `toml:"render"`
	UI        UI        `toml:"ui"`
	Vault     Vault     `toml:"vault"`
	Device    Device    `toml:"device"`
}

type Session struct {
	Idle time.Duration `toml:"idle"`
	Hard time.Duration `toml:"hard"`
}

type Clipboard struct {
	Clear time.Duration `toml:"clear"`
}

type Sync struct {
	Stale   time.Duration `toml:"stale"`
	After   time.Duration `toml:"after"`
	Every   time.Duration `toml:"every"`
	Sign    Sign          `toml:"sign"`
	SignKey string        `toml:"signkey"`
	Name    string        `toml:"name"`
	Email   string        `toml:"email"`
}

type UI struct {
	Mode      Mode `toml:"mode"`
	AltScreen bool `toml:"altscreen"`
	Height    int  `toml:"height"`
}

type Vault struct {
	History int    `toml:"history"`
	Machine string `toml:"machine"`
}

type Device struct {
	MaxAge time.Duration `toml:"max_age"`
}

type Render struct {
	Shell   render.Shell      `toml:"shell"`
	Formats map[string]string `toml:"formats"`
}

func Default() Config {
	return Config{
		Session:   Session{Idle: 15 * time.Minute, Hard: 4 * time.Hour},
		Clipboard: Clipboard{Clear: 30 * time.Second},
		Sync:      Sync{Stale: 5 * time.Minute, After: 2 * time.Second, Every: 30 * time.Second, Sign: SignOff},
		Render:    Render{Shell: render.DefaultShell(), Formats: map[string]string{}},
		UI:        UI{Mode: ModeFullscreen, AltScreen: true, Height: DefaultHeight},
		Vault:     Vault{History: vaultfiles.DefaultHistory},
		Device:    Device{MaxAge: 72 * time.Hour},
	}
}
