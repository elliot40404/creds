package session

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/elliot40404/creds/internal/keystore"
)

func benchStore(b *testing.B, protect bool) *Store {
	b.Helper()
	now := time.Now()
	s := &Store{
		Path: filepath.Join(b.TempDir(), "session"),
		Idle: time.Hour,
		Hard: time.Hour,
		Now:  func() time.Time { return now },
	}
	if protect {
		keys, err := keystore.Default()
		if err != nil {
			b.Skipf("no key store: %v", err)
		}
		s.Keys = keys
		b.Cleanup(func() { _ = s.Delete() })
	}
	if err := s.Save(secretID); err != nil {
		b.Fatal(err)
	}
	return s
}

func benchLoad(b *testing.B, protect bool) {
	s := benchStore(b, protect)
	for b.Loop() {
		if _, err := s.Load(); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkRevealProtected(b *testing.B) {
	s := benchStore(b, true)
	for b.Loop() {
		r, _, err := s.live(s.now())
		if err != nil {
			b.Fatal(err)
		}
		if _, err := s.reveal(r); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkLoadPlain(b *testing.B) { benchLoad(b, false) }

func BenchmarkLoadProtected(b *testing.B) { benchLoad(b, true) }
