package config

import (
	"errors"
	"strings"
	"testing"
)

func TestSignRoundTrip(t *testing.T) {
	for _, mode := range Signs() {
		c, err := Set(Default(), "sync.sign", mode)
		if err != nil {
			t.Fatalf("%s: %v", mode, err)
		}
		if got := string(c.Sync.Sign); got != mode {
			t.Fatalf("got %q want %q", got, mode)
		}
		if err := c.validate(); err != nil {
			t.Fatalf("%s: %v", mode, err)
		}
	}
	c, err := Set(Default(), "sync.signkey", "  ~/.ssh/id_ed25519.pub  ")
	if err != nil {
		t.Fatal(err)
	}
	if c.Sync.SignKey != "~/.ssh/id_ed25519.pub" {
		t.Fatalf("signkey %q", c.Sync.SignKey)
	}
}

func TestSignRejectsUnknownMode(t *testing.T) {
	c := Default()
	c.Sync.Sign = "gpg"
	err := c.validate()
	if !errors.Is(err, ErrUnknownSign) {
		t.Fatalf("err %v", err)
	}
	if !strings.Contains(err.Error(), "off, ssh, inherit") {
		t.Fatalf("message does not list the modes: %v", err)
	}
}

func TestSignDefaultsToOff(t *testing.T) {
	if Default().Sync.Sign != SignOff {
		t.Fatal("signing is on by default, that breaks commits for anyone without a key")
	}
}

func TestSignKeyRejectsControlCharacters(t *testing.T) {
	c := Default()
	c.Sync.SignKey = "key\x00path"
	if err := c.validate(); err == nil {
		t.Fatal("control characters accepted")
	}
}

func TestCommitIdentityFallsBackToTheFixedOne(t *testing.T) {
	c := Default()
	if c.Sync.Name != "" || c.Sync.Email != "" {
		t.Fatal("identity is not blank by default")
	}
	c, err := Set(c, "sync.email", "me@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if c.Sync.Email != "me@example.com" || c.Sync.Name != "" {
		t.Fatalf("email alone changed the name: %q <%q>", c.Sync.Name, c.Sync.Email)
	}
	if err := c.validate(); err != nil {
		t.Fatal(err)
	}
}

func TestCommitIdentityRejectsBrackets(t *testing.T) {
	for _, bad := range []string{"me <me@example.com>", "a\nb", "x>y"} {
		c := Default()
		c.Sync.Email = bad
		if err := c.validate(); err == nil {
			t.Fatalf("accepted %q", bad)
		}
	}
}
