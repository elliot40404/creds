package crypto

import (
	"errors"
	"regexp"
	"testing"
)

var recoveryRe = regexp.MustCompile(`^[A-Z2-7]{5}(-[A-Z2-7]{5}){5}$`)

func TestNewRecoveryCode(t *testing.T) {
	seen := map[string]bool{}
	for range 50 {
		code, err := NewRecoveryCode()
		if err != nil {
			t.Fatal(err)
		}
		if !recoveryRe.MatchString(code) {
			t.Fatalf("bad format %q", code)
		}
		if seen[code] {
			t.Fatal("duplicate code")
		}
		seen[code] = true
	}
}

func TestNormalizeRecoveryCode(t *testing.T) {
	cases := map[string]string{
		"ABCDE-FGHIJ":        "ABCDEFGHIJ",
		" abcde fghij\n":     "ABCDEFGHIJ",
		"abcde-\tFGHIJ-2345": "ABCDEFGHIJ2345",
	}
	for in, want := range cases {
		if got := NormalizeRecoveryCode(in); got != want {
			t.Fatalf("%q: got %q want %q", in, got, want)
		}
	}
}

func TestUnwrapWrongRecoveryCode(t *testing.T) {
	code, err := NewRecoveryCode()
	if err != nil {
		t.Fatal(err)
	}
	other, err := NewRecoveryCode()
	if err != nil {
		t.Fatal(err)
	}
	id, err := NewIdentity()
	if err != nil {
		t.Fatal(err)
	}
	data, err := WrapIdentity(id, NormalizeRecoveryCode(code), testLogN)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := UnwrapIdentity(data, NormalizeRecoveryCode(other)); !errors.Is(err, ErrWrongSecret) {
		t.Fatalf("got %v", err)
	}
	typed := " " + code[:10] + "  " + code[10:] + " "
	got, err := UnwrapIdentity(data, NormalizeRecoveryCode(typed))
	if err != nil {
		t.Fatal(err)
	}
	if got.String() != id.String() {
		t.Fatal("identity mismatch")
	}
}

func TestCanonicalRecoveryCode(t *testing.T) {
	if got := CanonicalRecoveryCode(" abcde fghij-klm "); got != "ABCDE-FGHIJ-KLM" {
		t.Fatalf("got %q", got)
	}
}
