package session

import (
	"os"
	"testing"
)

func TestLoadSurvivesRefreshBlockedByReader(t *testing.T) {
	t.Parallel()
	s, clock := newStore(t)
	if err := s.Save("AGE-SECRET-KEY-1TEST"); err != nil {
		t.Fatal(err)
	}
	before := readRecord(t, s.Path)
	f, err := os.Open(s.Path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	clock.advance(s.Idle / 2)
	got, err := s.Load()
	if err != nil || got != "AGE-SECRET-KEY-1TEST" {
		t.Fatalf("got %q, %v", got, err)
	}
	if after := readRecord(t, s.Path); !after.LastUsed.Equal(before.LastUsed) {
		t.Fatal("blocked refresh changed the session")
	}
}
