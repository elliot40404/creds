package session

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/elliot40404/creds/internal/fsutil"
	"github.com/elliot40404/creds/internal/keystore"
)

const secretID = "AGE-SECRET-KEY-1PROTECTED"

type fakeKeys struct {
	mu       sync.Mutex
	sealErr  error
	openErr  error
	seals    int
	forgets  []string
	accounts []string
}

func (k *fakeKeys) Method() string { return "fake" }

func (k *fakeKeys) Seal(account string, plain []byte) ([]byte, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.seals++
	k.accounts = append(k.accounts, account)
	if k.sealErr != nil {
		return nil, k.sealErr
	}
	tag := sha256.Sum256([]byte(account))
	return append(tag[:8], flip(plain)...), nil
}

func (k *fakeKeys) Open(account string, sealed []byte) ([]byte, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.accounts = append(k.accounts, account)
	tag := sha256.Sum256([]byte(account))
	if k.openErr != nil {
		return nil, k.openErr
	}
	if len(sealed) < 8 || !bytes.Equal(sealed[:8], tag[:8]) {
		return nil, keystore.ErrOpen
	}
	return flip(sealed[8:]), nil
}

func (k *fakeKeys) Forget(account string) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.forgets = append(k.forgets, account)
	return nil
}

func flip(b []byte) []byte {
	out := make([]byte, len(b))
	for i, c := range b {
		out[i] = c ^ 0x5a
	}
	return out
}

func protectedStore(t *testing.T) (*Store, *fakeClock, *fakeKeys) {
	t.Helper()
	s, clock := newStore(t)
	keys := &fakeKeys{}
	s.Keys = keys
	return s, clock, keys
}

func rawFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestSaveSealsIdentity(t *testing.T) {
	s, _, _ := protectedStore(t)
	if err := s.Save(secretID); err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(rawFile(t, s.Path), []byte(secretID)) {
		t.Fatal("plaintext identity in the session file")
	}
	r := readRecord(t, s.Path)
	if r.Version != version || r.Method != "fake" || r.Identity != "" || len(r.Sealed) == 0 {
		t.Fatalf("got %+v", r)
	}
}

func TestProtectedRoundTrip(t *testing.T) {
	s, clock, keys := protectedStore(t)
	saveAndAdvance(t, s, clock, time.Minute)
	sealed := readRecord(t, s.Path).Sealed
	id, err := s.Load()
	if err != nil || id != "id" {
		t.Fatalf("got %q %v", id, err)
	}
	r := readRecord(t, s.Path)
	if !r.LastUsed.Equal(clock.t) || !bytes.Equal(r.Sealed, sealed) || keys.seals != 1 {
		t.Fatalf("refresh resealed or missed: %+v seals %d", r, keys.seals)
	}
}

func TestAccountStable(t *testing.T) {
	s, _, keys := protectedStore(t)
	if err := s.Save(secretID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Load(); err != nil {
		t.Fatal(err)
	}
	if err := s.Delete(); err != nil {
		t.Fatal(err)
	}
	acct := keys.accounts[0]
	if len(acct) != 64 || keys.accounts[1] != acct || keys.forgets[0] != acct {
		t.Fatalf("accounts %v forgets %v", keys.accounts, keys.forgets)
	}
	other, _, _ := protectedStore(t)
	if other.account() == acct {
		t.Fatal("two homes share an account")
	}
}

func TestSavePlainFallback(t *testing.T) {
	for name, cause := range map[string]error{
		"unavailable": keystore.ErrUnavailable,
		"other":       errors.New("boom"),
	} {
		t.Run(name, func(t *testing.T) {
			s, _, keys := protectedStore(t)
			keys.sealErr = cause
			err := s.Save(secretID)
			if !errors.Is(err, ErrPlainFallback) || !errors.Is(err, cause) {
				t.Fatalf("err = %v", err)
			}
			r := readRecord(t, s.Path)
			if r.Method != methodPlain || r.Identity != secretID || len(r.Sealed) != 0 {
				t.Fatalf("got %+v", r)
			}
			if id, err := s.Load(); err != nil || id != secretID {
				t.Fatalf("got %q %v", id, err)
			}
		})
	}
}

func TestSavePlainWithoutKeys(t *testing.T) {
	s, _ := newStore(t)
	if err := s.Save(secretID); err != nil {
		t.Fatal(err)
	}
	if r := readRecord(t, s.Path); r.Version != version || r.Method != methodPlain || r.Identity != secretID {
		t.Fatalf("got %+v", r)
	}
}

func writeV1(t *testing.T, s *Store) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(s.Path), 0o700); err != nil {
		t.Fatal(err)
	}
	body := `{"identity":"` + secretID + `","created":"2026-01-01T11:59:00Z","last_used":"2026-01-01T11:59:00Z"}`
	if err := fsutil.WriteFileAtomic(s.Path, []byte(body)); err != nil {
		t.Fatal(err)
	}
}

func TestLoadV1(t *testing.T) {
	s, clock := newStore(t)
	writeV1(t, s)
	if id, err := s.Load(); err != nil || id != secretID {
		t.Fatalf("got %q %v", id, err)
	}
	if r := readRecord(t, s.Path); !r.LastUsed.Equal(clock.t) {
		t.Fatalf("got %+v", r)
	}
}

func TestLoadUpgradesPlain(t *testing.T) {
	s, clock, _ := protectedStore(t)
	writeV1(t, s)
	if id, err := s.Load(); err != nil || id != secretID {
		t.Fatalf("got %q %v", id, err)
	}
	r := readRecord(t, s.Path)
	if r.Method != "fake" || r.Identity != "" || !r.LastUsed.Equal(clock.t) || r.Created.Equal(clock.t) {
		t.Fatalf("not upgraded: %+v", r)
	}
	if id, err := s.Load(); err != nil || id != secretID {
		t.Fatalf("after upgrade %q %v", id, err)
	}
}

func TestLoadUpgradeFailureKeepsPlain(t *testing.T) {
	s, clock, keys := protectedStore(t)
	writeV1(t, s)
	keys.sealErr = keystore.ErrUnavailable
	if id, err := s.Load(); err != nil || id != secretID {
		t.Fatalf("got %q %v", id, err)
	}
	if r := readRecord(t, s.Path); r.Identity != secretID || !r.LastUsed.Equal(clock.t) {
		t.Fatalf("got %+v", r)
	}
}

func TestLoadOpenFailureDiscards(t *testing.T) {
	cases := map[string]func(*Store, *fakeKeys){
		"open":     func(_ *Store, k *fakeKeys) { k.openErr = keystore.ErrOpen },
		"no keys":  func(s *Store, _ *fakeKeys) { s.Keys = nil },
		"moved":    func(s *Store, _ *fakeKeys) { s.Path += "x" },
		"other os": func(s *Store, _ *fakeKeys) { s.Keys = otherKeys{s.Keys} },
	}
	for name, breakIt := range cases {
		t.Run(name, func(t *testing.T) {
			s, _, keys := protectedStore(t)
			if err := s.Save(secretID); err != nil {
				t.Fatal(err)
			}
			if name == "moved" {
				if err := os.Rename(s.Path, s.Path+"x"); err != nil {
					t.Fatal(err)
				}
			}
			breakIt(s, keys)
			if _, err := s.Load(); !errors.Is(err, ErrNoSession) {
				t.Fatalf("err = %v", err)
			}
			assertGone(t, s.Path)
		})
	}
}

type otherKeys struct{ keystore.Store }

func (otherKeys) Method() string { return "other" }

func TestDeleteForgets(t *testing.T) {
	s, _, keys := protectedStore(t)
	if err := s.Save(secretID); err != nil {
		t.Fatal(err)
	}
	if err := s.Delete(); err != nil {
		t.Fatal(err)
	}
	if err := s.Delete(); err != nil {
		t.Fatal(err)
	}
	assertGone(t, s.Path)
	if len(keys.forgets) != 2 {
		t.Fatalf("forgets %v", keys.forgets)
	}
}

func TestExpiryForgets(t *testing.T) {
	s, clock, keys := protectedStore(t)
	saveAndAdvance(t, s, clock, s.Idle)
	if _, err := s.Load(); !errors.Is(err, ErrExpired) {
		t.Fatalf("err = %v", err)
	}
	if len(keys.forgets) != 1 {
		t.Fatalf("forgets %v", keys.forgets)
	}
}
