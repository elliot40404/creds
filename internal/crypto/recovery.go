package crypto

import (
	"crypto/rand"
	"encoding/base32"
	"strings"
)

const (
	recoveryChars = 30
	recoveryGroup = 5
)

func NewRecoveryCode() (string, error) {
	buf := make([]byte, 19)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return group(base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(buf)[:recoveryChars]), nil
}

func CanonicalRecoveryCode(s string) string {
	return group(NormalizeRecoveryCode(s))
}

func group(raw string) string {
	groups := make([]string, 0, len(raw)/recoveryGroup+1)
	for len(raw) > recoveryGroup {
		groups = append(groups, raw[:recoveryGroup])
		raw = raw[recoveryGroup:]
	}
	return strings.Join(append(groups, raw), "-")
}

func NormalizeRecoveryCode(s string) string {
	return strings.ToUpper(strings.Map(func(r rune) rune {
		if r == '-' || r == ' ' || r == '\t' || r == '\n' || r == '\r' {
			return -1
		}
		return r
	}, s))
}
