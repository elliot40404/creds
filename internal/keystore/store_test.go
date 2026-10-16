//go:build windows || linux

package keystore_test

import (
	"bytes"
	"crypto/rand"
	"errors"
	"strings"
	"testing"

	"github.com/elliot40404/creds/internal/keystore"
)

const identity = "AGE-SECRET-KEY-1QQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQ"

func account(t testing.TB, s keystore.Store) string {
	t.Helper()
	name := "test-" + t.Name() + "-" + rand.Text()
	t.Cleanup(func() {
		if err := s.Forget(name); err != nil {
			t.Error(err)
		}
	})
	return name
}

func seal(t testing.TB, s keystore.Store, acct string, plain []byte) []byte {
	t.Helper()
	sealed, err := s.Seal(acct, plain)
	if err != nil {
		t.Fatal(err)
	}
	return sealed
}

func TestMethod(t *testing.T) {
	t.Parallel()
	if got := store(t).Method(); got != wantMethod {
		t.Fatalf("method %q", got)
	}
}

func TestRoundTrip(t *testing.T) {
	t.Parallel()
	s := store(t)
	acct := account(t, s)
	for _, plain := range [][]byte{[]byte(identity), {}, bytes.Repeat([]byte{0}, 4096)} {
		sealed := seal(t, s, acct, plain)
		if bytes.Contains(sealed, []byte(identity)) {
			t.Fatal("sealed blob holds the plaintext")
		}
		got, err := s.Open(acct, sealed)
		if err != nil || !bytes.Equal(got, plain) {
			t.Fatalf("open %d bytes: %v", len(plain), err)
		}
	}
}

func TestWrongAccountFails(t *testing.T) {
	t.Parallel()
	s := store(t)
	other := account(t, s)
	seal(t, s, other, []byte(identity))
	sealed := seal(t, s, account(t, s), []byte(identity))
	if _, err := s.Open(other, sealed); !errors.Is(err, keystore.ErrOpen) {
		t.Fatalf("got %v", err)
	}
}

func TestTamperFails(t *testing.T) {
	t.Parallel()
	s := store(t)
	acct := account(t, s)
	sealed := seal(t, s, acct, []byte(identity))
	for _, at := range []int{0, len(sealed) / 2, len(sealed) - 1} {
		bad := bytes.Clone(sealed)
		bad[at] ^= 0x01
		if _, err := s.Open(acct, bad); !errors.Is(err, keystore.ErrOpen) {
			t.Fatalf("flip at %d: %v", at, err)
		}
	}
	for _, bad := range [][]byte{nil, sealed[:len(sealed)-1]} {
		if _, err := s.Open(acct, bad); !errors.Is(err, keystore.ErrOpen) {
			t.Fatalf("%d bytes: %v", len(bad), err)
		}
	}
}

func TestBadInputRejected(t *testing.T) {
	t.Parallel()
	s := store(t)
	acct := account(t, s)
	if _, err := s.Seal(acct, make([]byte, keystore.MaxPlain+1)); err == nil {
		t.Fatal("seal over the limit")
	}
	if _, err := s.Seal(strings.Repeat("a", 257), []byte(identity)); err == nil {
		t.Fatal("seal with a long account")
	}
	if _, err := s.Open(acct, make([]byte, 4*keystore.MaxPlain)); !errors.Is(err, keystore.ErrOpen) {
		t.Fatalf("open over the limit: %v", err)
	}
	if _, err := s.Seal("", []byte(identity)); err == nil {
		t.Fatal("seal with empty account")
	}
	if _, err := s.Open("", []byte{1}); err == nil || errors.Is(err, keystore.ErrOpen) {
		t.Fatalf("open with empty account: %v", err)
	}
	if err := s.Forget(""); err == nil {
		t.Fatal("forget with empty account")
	}
}

func BenchmarkSeal(b *testing.B) {
	s := store(b)
	acct := account(b, s)
	for b.Loop() {
		seal(b, s, acct, []byte(identity))
	}
}

func BenchmarkOpen(b *testing.B) {
	s := store(b)
	acct := account(b, s)
	sealed := seal(b, s, acct, []byte(identity))
	for b.Loop() {
		if _, err := s.Open(acct, sealed); err != nil {
			b.Fatal(err)
		}
	}
}
