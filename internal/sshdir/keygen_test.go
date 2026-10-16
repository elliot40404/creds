package sshdir

import (
	"bytes"
	"crypto/ed25519"
	"errors"
	"testing"

	"golang.org/x/crypto/ssh"
)

func TestNewKeyParses(t *testing.T) {
	t.Parallel()
	priv, pub, err := NewKey("work", nil)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := ssh.ParseRawPrivateKey(priv)
	if err != nil {
		t.Fatal(err)
	}
	if Classify("work", priv) != KindPrivateKey || Classify("work.pub", pub) != KindPublicKey || !IsPublicKey(pub) {
		t.Fatal("not classified as a key pair")
	}
	matchesPub(t, raw, pub, "work")
}

func TestNewKeyWithPassphrase(t *testing.T) {
	t.Parallel()
	priv, pub, err := NewKey("", []byte("open sesame"))
	if err != nil {
		t.Fatal(err)
	}
	var missing *ssh.PassphraseMissingError
	if _, err := ssh.ParseRawPrivateKey(priv); !errors.As(err, &missing) {
		t.Fatalf("no passphrase: %v", err)
	}
	if _, err := ssh.ParseRawPrivateKeyWithPassphrase(priv, []byte("wrong")); err == nil {
		t.Fatal("wrong passphrase accepted")
	}
	raw, err := ssh.ParseRawPrivateKeyWithPassphrase(priv, []byte("open sesame"))
	if err != nil {
		t.Fatal(err)
	}
	if Classify("k", priv) != KindPrivateKey {
		t.Fatal("not a private key")
	}
	matchesPub(t, raw, pub, "")
}

func TestNewKeyIsRandom(t *testing.T) {
	t.Parallel()
	a, _, errA := NewKey("", nil)
	b, _, errB := NewKey("", nil)
	if errA != nil || errB != nil || bytes.Equal(a, b) {
		t.Fatal("same key twice")
	}
}

func matchesPub(t *testing.T, raw any, pub []byte, comment string) {
	t.Helper()
	key, ok := raw.(*ed25519.PrivateKey)
	if !ok {
		t.Fatalf("type %T", raw)
	}
	want, err := ssh.NewPublicKey(key.Public())
	if err != nil {
		t.Fatal(err)
	}
	got, gotComment, _, rest, err := ssh.ParseAuthorizedKey(pub)
	if err != nil || len(rest) != 0 || gotComment != comment || !bytes.Equal(got.Marshal(), want.Marshal()) {
		t.Fatalf("pub %q: %v %q", pub, err, gotComment)
	}
}
