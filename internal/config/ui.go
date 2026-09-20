package config

import (
	"errors"
	"fmt"
	"slices"
	"strings"
)

type Mode string

const (
	ModeFullscreen Mode = "fullscreen"
	ModeInline     Mode = "inline"
)

const (
	MinHeight     = 10
	MaxHeight     = 60
	DefaultHeight = 15
)

var ErrUnknownMode = errors.New("not a ui mode")

func Modes() []string {
	return []string{string(ModeFullscreen), string(ModeInline)}
}

func (m Mode) valid() bool {
	return slices.Contains(Modes(), string(m))
}

func checkUI(u UI) error {
	if !u.Mode.valid() {
		return fmt.Errorf("ui.mode: %w: %q, want one of %s", ErrUnknownMode, string(u.Mode), strings.Join(Modes(), ", "))
	}
	if u.Height < MinHeight || u.Height > MaxHeight {
		return fmt.Errorf("ui.height must be between %d and %d, got %d", MinHeight, MaxHeight, u.Height)
	}
	return nil
}
