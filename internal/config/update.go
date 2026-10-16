package config

import (
	"errors"
	"fmt"
	"slices"
	"strings"
)

type UpdateMode string

const (
	UpdateHint    UpdateMode = "hint"
	UpdateInstall UpdateMode = "install"
	UpdateOff     UpdateMode = "off"
)

var ErrUnknownUpdate = errors.New("not an update mode")

type Update struct {
	Mode UpdateMode `toml:"mode,omitempty"`
}

func UpdateModes() []string {
	return []string{string(UpdateHint), string(UpdateInstall), string(UpdateOff)}
}

func (u Update) Current() UpdateMode {
	if u.Mode == "" {
		return UpdateHint
	}
	return u.Mode
}

func checkUpdate(u Update) error {
	if !slices.Contains(UpdateModes(), string(u.Current())) {
		return fmt.Errorf("update.mode: %w: %q, want one of %s", ErrUnknownUpdate, string(u.Mode), strings.Join(UpdateModes(), ", "))
	}
	return nil
}

func updateField() Field {
	f := choice("update.mode", "when a new creds release is out: hint prints a note, install also installs it, off never checks",
		UpdateModes(), func(c *Config) *UpdateMode { return &c.Update.Mode })
	f.get = func(c *Config) string { return string(c.Update.Current()) }
	return f
}
