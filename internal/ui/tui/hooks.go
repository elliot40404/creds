package tui

import (
	tea "charm.land/bubbletea/v2"
	"github.com/elliot40404/creds/internal/vault"
)

type Screen interface {
	Update(msg tea.Msg) (Screen, tea.Cmd)
	View(width, height int) string
}

type FormResult struct {
	Entry    vault.Entry
	Canceled bool
}

type FormError struct {
	Err error
}

type PasswordResult struct {
	Password string
	Canceled bool
}

type PasswordError struct {
	Err error
}

type Hooks struct {
	Add    func(paths []string) Screen
	Edit   func(orig vault.Entry) Screen
	Prompt func(message string) Screen
}
