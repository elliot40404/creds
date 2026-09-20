package crypto

import (
	"bytes"
	"testing"
)

func TestMACKey(t *testing.T) {
	a, err := NewIdentity()
	if err != nil {
		t.Fatal(err)
	}
	b, err := NewIdentity()
	if err != nil {
		t.Fatal(err)
	}
	ka := mustMACKey(t, a)
	if len(ka) != 32 {
		t.Fatalf("key len %d", len(ka))
	}
	parsed, err := ParseIdentity(a.String())
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(ka, mustMACKey(t, parsed)) {
		t.Fatal("key not stable across parse")
	}
	if bytes.Equal(ka, mustMACKey(t, b)) {
		t.Fatal("distinct identities share key")
	}
}

func mustMACKey(t *testing.T, id *Identity) []byte {
	t.Helper()
	k, err := id.MACKey()
	if err != nil {
		t.Fatal(err)
	}
	return k
}
