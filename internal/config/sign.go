package config

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/elliot40404/creds/internal/safetext"
)

type Sign string

const (
	SignOff     Sign = "off"
	SignSSH     Sign = "ssh"
	SignInherit Sign = "inherit"
)

const (
	maxSignKey  = 4096
	maxIdentity = 256
)

var ErrUnknownSign = errors.New("not a signing mode")

func Signs() []string {
	return []string{string(SignOff), string(SignSSH), string(SignInherit)}
}

func (s Sign) valid() bool {
	return slices.Contains(Signs(), string(s))
}

func checkSign(sy Sync) error {
	if !sy.Sign.valid() {
		return fmt.Errorf("sync.sign: %w: %q, want one of %s", ErrUnknownSign, string(sy.Sign), strings.Join(Signs(), ", "))
	}
	if safetext.HasControl(sy.SignKey) || len(sy.SignKey) > maxSignKey {
		return fmt.Errorf("sync.signkey must be plain text of at most %d characters", maxSignKey)
	}
	for _, f := range [][2]string{{"sync.name", sy.Name}, {"sync.email", sy.Email}} {
		if safetext.HasControl(f[1]) || strings.ContainsAny(f[1], "<>\n") || len(f[1]) > maxIdentity {
			return fmt.Errorf("%s must be plain text of at most %d characters, without < or >", f[0], maxIdentity)
		}
	}
	return nil
}
