package crypto

import (
	"errors"
	"testing"
)

const (
	testLogN   = 2
	testSecret = "correct horse battery"
)

func newWrapped(t *testing.T) (*Identity, []byte) {
	t.Helper()
	id, err := NewIdentity()
	if err != nil {
		t.Fatal(err)
	}
	data, err := WrapIdentity(id, testSecret, testLogN)
	if err != nil {
		t.Fatal(err)
	}
	return id, data
}

func TestWrapRoundtrip(t *testing.T) {
	id, data := newWrapped(t)
	got, err := UnwrapIdentity(data, testSecret)
	if err != nil {
		t.Fatal(err)
	}
	if got.String() != id.String() {
		t.Fatal("identity mismatch")
	}
}

func TestUnwrapWrongSecret(t *testing.T) {
	_, data := newWrapped(t)
	for _, s := range []string{"wrong horse battery", ""} {
		if _, err := UnwrapIdentity(data, s); !errors.Is(err, ErrWrongSecret) {
			t.Fatalf("secret %q: got %v", s, err)
		}
	}
}

const (
	nfcWord = "caf" + string(rune(0xe9)) + " horse"
	nfdWord = "cafe" + string(rune(0x301)) + " horse"
)

func TestWrapNormalizesSecret(t *testing.T) {
	id, err := NewIdentity()
	if err != nil {
		t.Fatal(err)
	}
	data, err := WrapIdentity(id, nfdWord, testLogN)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := unwrap(data, nfcWord); err != nil {
		t.Fatalf("stored form not NFC: %v", err)
	}
	for _, s := range []string{nfcWord, nfdWord} {
		got, err := UnwrapIdentity(data, s)
		if err != nil || got.String() != id.String() {
			t.Fatalf("secret %q: %v", s, err)
		}
	}
}

func TestUnwrapLegacyRawSecret(t *testing.T) {
	id, err := NewIdentity()
	if err != nil {
		t.Fatal(err)
	}
	data, err := wrap(id, nfdWord, testLogN)
	if err != nil {
		t.Fatal(err)
	}
	got, err := UnwrapIdentity(data, nfdWord)
	if err != nil || got.String() != id.String() {
		t.Fatalf("legacy: %v", err)
	}
}

func TestUnwrapCorrupted(t *testing.T) {
	_, data := newWrapped(t)
	data[len(data)-1] ^= 0xff
	_, err := UnwrapIdentity(data, testSecret)
	if err == nil || errors.Is(err, ErrWrongSecret) {
		t.Fatalf("got %v", err)
	}
}

func TestUnwrapTruncated(t *testing.T) {
	_, data := newWrapped(t)
	for _, n := range []int{0, 10, len(data) / 2, len(data) - 1} {
		if _, err := UnwrapIdentity(data[:n], testSecret); err == nil {
			t.Fatalf("len %d: expected error", n)
		}
	}
}
