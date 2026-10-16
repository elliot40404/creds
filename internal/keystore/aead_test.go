package keystore

import (
	"bytes"
	"errors"
	"sync"
	"testing"
)

type memSource struct {
	mu   sync.Mutex
	keys map[string][]byte
}

func (m *memSource) key(name string, create bool) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	k, ok := m.keys[name]
	if !ok && !create {
		return nil, errNoKey
	}
	if !ok {
		k = newKey()
		m.keys[name] = k
	}
	return bytes.Clone(k), nil
}

func (m *memSource) forget(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.keys, name)
	return nil
}

const secret = "AGE-SECRET-KEY-1QQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQ"

func memStore() (aeadStore, *memSource) {
	src := &memSource{keys: map[string][]byte{}}
	return aeadStore{method: "mem", src: src}, src
}

func mustSeal(t *testing.T, s aeadStore) []byte {
	t.Helper()
	sealed, err := s.Seal("home-a", []byte(secret))
	if err != nil {
		t.Fatal(err)
	}
	return sealed
}

func TestAEADRoundTrip(t *testing.T) {
	t.Parallel()
	s, _ := memStore()
	for _, plain := range [][]byte{[]byte(secret), {}, make([]byte, MaxPlain)} {
		sealed, err := s.Seal("home-a", plain)
		if err != nil {
			t.Fatal(err)
		}
		if len(plain) > 0 && bytes.Contains(sealed, plain) {
			t.Fatal("sealed blob holds the plaintext")
		}
		got, err := s.Open("home-a", sealed)
		if err != nil || !bytes.Equal(got, plain) {
			t.Fatalf("open %d bytes: %v", len(plain), err)
		}
	}
}

func TestAEADWrongAccountFailsWithSameKey(t *testing.T) {
	t.Parallel()
	s, src := memStore()
	sealed := mustSeal(t, s)
	src.keys[keyName("home-b")] = bytes.Clone(src.keys[keyName("home-a")])
	if _, err := s.Open("home-b", sealed); !errors.Is(err, ErrOpen) {
		t.Fatalf("got %v", err)
	}
}

func TestAEADTamperFails(t *testing.T) {
	t.Parallel()
	s, _ := memStore()
	sealed := mustSeal(t, s)
	for at := range sealed {
		bad := bytes.Clone(sealed)
		bad[at] ^= 0x80
		if _, err := s.Open("home-a", bad); !errors.Is(err, ErrOpen) {
			t.Fatalf("flip at %d: %v", at, err)
		}
	}
	for _, n := range []int{0, 1, 12, 28, len(sealed) - 1} {
		if _, err := s.Open("home-a", sealed[:n]); !errors.Is(err, ErrOpen) {
			t.Fatalf("%d bytes: %v", n, err)
		}
	}
}

func TestAEADForgetDropsKey(t *testing.T) {
	t.Parallel()
	s, _ := memStore()
	sealed := mustSeal(t, s)
	if err := s.Forget("home-a"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Open("home-a", sealed); !errors.Is(err, ErrOpen) || !errors.Is(err, errNoKey) {
		t.Fatalf("got %v", err)
	}
	if err := s.Forget(""); !errors.Is(err, errAccount) {
		t.Fatalf("got %v", err)
	}
}

func TestAEADBadKeySize(t *testing.T) {
	t.Parallel()
	s, src := memStore()
	sealed := mustSeal(t, s)
	src.keys[keyName("home-a")] = make([]byte, 16)
	if _, err := s.Open("home-a", sealed); !errors.Is(err, ErrOpen) || !errors.Is(err, errKeySize) {
		t.Fatalf("got %v", err)
	}
	if _, err := s.Seal("home-a", nil); !errors.Is(err, errKeySize) {
		t.Fatalf("got %v", err)
	}
}

func TestKeyNameIsFixedHex(t *testing.T) {
	t.Parallel()
	a, b := keyName("home-a\n delete-keychain"), keyName("home-b")
	if len(a) != 64 || a == b || bytes.ContainsAny([]byte(a), " \n\"'") {
		t.Fatalf("names %q %q", a, b)
	}
}
