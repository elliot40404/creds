package keystore_test

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/elliot40404/creds/internal/keystore"
)

const identity = "AGE-SECRET-KEY-1QQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQ"

func store(t testing.TB) keystore.Store {
	t.Helper()
	s, err := keystore.Default()
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func seal(t testing.TB, s keystore.Store, plain []byte) []byte {
	t.Helper()
	sealed, err := s.Seal("home-a", plain)
	if err != nil {
		t.Fatal(err)
	}
	return sealed
}

func TestMethodIsDPAPI(t *testing.T) {
	t.Parallel()
	if got := store(t).Method(); got != "dpapi" {
		t.Fatalf("method %q", got)
	}
}

func TestRoundTrip(t *testing.T) {
	t.Parallel()
	s := store(t)
	for _, plain := range [][]byte{[]byte(identity), {}, bytes.Repeat([]byte{0}, 4096)} {
		sealed := seal(t, s, plain)
		if bytes.Contains(sealed, []byte(identity)) {
			t.Fatal("sealed blob holds the plaintext")
		}
		got, err := s.Open("home-a", sealed)
		if err != nil || !bytes.Equal(got, plain) {
			t.Fatalf("open %d bytes: %v", len(plain), err)
		}
	}
}

func TestWrongAccountFails(t *testing.T) {
	t.Parallel()
	s := store(t)
	sealed := seal(t, s, []byte(identity))
	if _, err := s.Open("home-b", sealed); !errors.Is(err, keystore.ErrOpen) {
		t.Fatalf("got %v", err)
	}
}

func TestTamperFails(t *testing.T) {
	t.Parallel()
	s := store(t)
	sealed := seal(t, s, []byte(identity))
	for _, at := range []int{0, len(sealed) / 2, len(sealed) - 1} {
		bad := bytes.Clone(sealed)
		bad[at] ^= 0x01
		if _, err := s.Open("home-a", bad); !errors.Is(err, keystore.ErrOpen) {
			t.Fatalf("flip at %d: %v", at, err)
		}
	}
	for _, bad := range [][]byte{nil, sealed[:len(sealed)-1]} {
		if _, err := s.Open("home-a", bad); !errors.Is(err, keystore.ErrOpen) {
			t.Fatalf("%d bytes: %v", len(bad), err)
		}
	}
}

func TestBadInputRejected(t *testing.T) {
	t.Parallel()
	s := store(t)
	if _, err := s.Seal("home-a", make([]byte, keystore.MaxPlain+1)); err == nil {
		t.Fatal("seal over the limit")
	}
	if _, err := s.Seal(strings.Repeat("a", 257), []byte(identity)); err == nil {
		t.Fatal("seal with a long account")
	}
	if _, err := s.Open("home-a", make([]byte, 4*keystore.MaxPlain)); !errors.Is(err, keystore.ErrOpen) {
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

func TestForgetKeepsBlobUsable(t *testing.T) {
	t.Parallel()
	s := store(t)
	sealed := seal(t, s, []byte(identity))
	if err := s.Forget("home-a"); err != nil {
		t.Fatal(err)
	}
	if err := s.Forget("home-a"); err != nil {
		t.Fatal(err)
	}
	if got, err := s.Open("home-a", sealed); err != nil || string(got) != identity {
		t.Fatalf("open after forget: %v", err)
	}
}

func BenchmarkSeal(b *testing.B) {
	s := store(b)
	for b.Loop() {
		seal(b, s, []byte(identity))
	}
}

func BenchmarkOpen(b *testing.B) {
	s := store(b)
	sealed := seal(b, s, []byte(identity))
	for b.Loop() {
		if _, err := s.Open("home-a", sealed); err != nil {
			b.Fatal(err)
		}
	}
}
