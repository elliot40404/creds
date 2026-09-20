package crypto

import (
	"strings"
	"testing"
)

func TestIdentityRoundtrip(t *testing.T) {
	id, err := NewIdentity()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(id.String(), "AGE-SECRET-KEY-PQ-1") {
		t.Fatal("unexpected identity prefix")
	}
	parsed, err := ParseIdentity(id.String())
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Recipient().String() != id.Recipient().String() {
		t.Fatal("recipient mismatch after parse")
	}
}

func TestParseInvalid(t *testing.T) {
	if _, err := ParseIdentity("AGE-SECRET-KEY-PQ-1BAD"); err == nil {
		t.Fatal("expected identity parse error")
	}
}
