package config

import (
	"fmt"
	"os"
	"unicode/utf8"

	"github.com/elliot40404/creds/internal/safetext"
)

const (
	MinHistory     = 0
	MaxHistory     = 50
	maxMachineName = 64
)

func DefaultMachine() string {
	name, err := os.Hostname()
	if err != nil || name == "" || safetext.HasControl(name) {
		return "unknown"
	}
	return trimName(name)
}

func trimName(s string) string {
	if len(s) <= maxMachineName {
		return s
	}
	end := maxMachineName
	for end > 0 && !utf8.RuneStart(s[end]) {
		end--
	}
	return s[:end]
}

func checkVault(v Vault) error {
	if v.History < MinHistory || v.History > MaxHistory {
		return fmt.Errorf("vault.history must be between %d and %d, got %d", MinHistory, MaxHistory, v.History)
	}
	if safetext.HasControl(v.Machine) || len(v.Machine) > maxMachineName {
		return fmt.Errorf("vault.machine must be plain text of at most %d characters", maxMachineName)
	}
	return nil
}

func (v Vault) MachineName() string {
	if v.Machine != "" {
		return v.Machine
	}
	return DefaultMachine()
}
