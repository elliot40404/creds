package crypto

import (
	"bytes"
	"errors"
	"testing"

	"filippo.io/age"
)

func newX25519(t *testing.T) *age.X25519Identity {
	t.Helper()
	k, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatal(err)
	}
	return k
}

func sealTo(t *testing.T, k *age.X25519Identity) (*Identity, []byte) {
	t.Helper()
	id, err := NewIdentity()
	if err != nil {
		t.Fatal(err)
	}
	data, err := SealIdentity(id, k.Recipient())
	if err != nil {
		t.Fatal(err)
	}
	return id, data
}

func TestSealIdentityRoundtrip(t *testing.T) {
	t.Parallel()
	k := newX25519(t)
	id, data := sealTo(t, k)
	got, err := OpenIdentity(data, k)
	if err != nil {
		t.Fatal(err)
	}
	if got.String() != id.String() {
		t.Fatal("identity mismatch")
	}
}

func TestOpenIdentityWrongKey(t *testing.T) {
	t.Parallel()
	_, data := sealTo(t, newX25519(t))
	if _, err := OpenIdentity(data, newX25519(t)); !errors.Is(err, ErrWrongKey) {
		t.Fatalf("got %v", err)
	}
}

func TestOpenIdentityRefusesJunk(t *testing.T) {
	t.Parallel()
	k := newX25519(t)
	junk := [][]byte{nil, []byte("junk"), append([]byte("age-encryption.org/v1\n"), "junk"...)}
	for _, data := range junk {
		if _, err := OpenIdentity(data, k); err == nil {
			t.Fatalf("%q: want error", data)
		}
	}
}

func TestOpenIdentityRefusesNonIdentity(t *testing.T) {
	t.Parallel()
	k := newX25519(t)
	var buf bytes.Buffer
	w, err := age.Encrypt(&buf, k.Recipient())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte("not an identity")); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := OpenIdentity(buf.Bytes(), k); err == nil {
		t.Fatal("want parse error")
	}
}
