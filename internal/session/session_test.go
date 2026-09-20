package session

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/elliot40404/creds/internal/fsutil"
)

type fakeClock struct{ t time.Time }

func (c *fakeClock) now() time.Time { return c.t }

func (c *fakeClock) advance(d time.Duration) { c.t = c.t.Add(d) }

func newStore(t *testing.T) (*Store, *fakeClock) {
	t.Helper()
	clock := &fakeClock{t: time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)}
	s := &Store{
		Path: filepath.Join(t.TempDir(), "sub", "session"),
		Idle: 15 * time.Minute,
		Hard: 4 * time.Hour,
		Now:  clock.now,
	}
	return s, clock
}

func readRecord(t *testing.T, path string) record {
	t.Helper()
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		t.Fatal(err)
	}
	var r record
	if err := json.Unmarshal(data, &r); err != nil {
		t.Fatal(err)
	}
	return r
}

func TestSaveWritesRecord(t *testing.T) {
	s, clock := newStore(t)
	if err := s.Save("AGE-SECRET-KEY-X"); err != nil {
		t.Fatal(err)
	}
	r := readRecord(t, s.Path)
	if r.Identity != "AGE-SECRET-KEY-X" || !r.Created.Equal(clock.t) || !r.LastUsed.Equal(clock.t) {
		t.Fatalf("got %+v", r)
	}
}

func TestSaveOverwrites(t *testing.T) {
	s, clock := newStore(t)
	if err := s.Save("one"); err != nil {
		t.Fatal(err)
	}
	clock.advance(time.Minute)
	if err := s.Save("two"); err != nil {
		t.Fatal(err)
	}
	r := readRecord(t, s.Path)
	if r.Identity != "two" || !r.Created.Equal(clock.t) {
		t.Fatalf("got %+v", r)
	}
}

func assertGone(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("session file still present: %v", err)
	}
}

func saveAndAdvance(t *testing.T, s *Store, clock *fakeClock, d time.Duration) {
	t.Helper()
	if err := s.Save("id"); err != nil {
		t.Fatal(err)
	}
	clock.advance(d)
}

func TestLoadNoSession(t *testing.T) {
	s, _ := newStore(t)
	if _, err := s.Load(); !errors.Is(err, ErrNoSession) {
		t.Fatalf("got %v", err)
	}
}

func TestLoadValid(t *testing.T) {
	s, clock := newStore(t)
	saveAndAdvance(t, s, clock, time.Minute)
	id, err := s.Load()
	if err != nil || id != "id" {
		t.Fatalf("got %q %v", id, err)
	}
	if r := readRecord(t, s.Path); !r.LastUsed.Equal(clock.t) {
		t.Fatalf("last_used not bumped: %+v", r)
	}
}

func TestLoadIdleExpiry(t *testing.T) {
	s, clock := newStore(t)
	saveAndAdvance(t, s, clock, 15*time.Minute)
	if _, err := s.Load(); !errors.Is(err, ErrExpired) {
		t.Fatalf("got %v", err)
	}
	assertGone(t, s.Path)
	if _, err := s.Load(); !errors.Is(err, ErrNoSession) {
		t.Fatalf("got %v", err)
	}
}

func TestLoadSlidingRefresh(t *testing.T) {
	s, clock := newStore(t)
	saveAndAdvance(t, s, clock, 0)
	for range 5 {
		clock.advance(10 * time.Minute)
		if _, err := s.Load(); err != nil {
			t.Fatal(err)
		}
	}
}

func TestLoadHardCap(t *testing.T) {
	s, clock := newStore(t)
	saveAndAdvance(t, s, clock, 0)
	for clock.t.Sub(readRecord(t, s.Path).Created) < s.Hard-10*time.Minute {
		clock.advance(10 * time.Minute)
		if _, err := s.Load(); err != nil {
			t.Fatal(err)
		}
	}
	clock.advance(10 * time.Minute)
	if _, err := s.Load(); !errors.Is(err, ErrExpired) {
		t.Fatalf("got %v", err)
	}
	assertGone(t, s.Path)
}

func TestDelete(t *testing.T) {
	s, clock := newStore(t)
	saveAndAdvance(t, s, clock, 0)
	if err := s.Delete(); err != nil {
		t.Fatal(err)
	}
	assertGone(t, s.Path)
	if _, err := s.Load(); !errors.Is(err, ErrNoSession) {
		t.Fatalf("got %v", err)
	}
	if err := s.Delete(); err != nil {
		t.Fatalf("delete missing: %v", err)
	}
}

func TestLoadCorrupt(t *testing.T) {
	cases := map[string]string{
		"empty":         ``,
		"garbage":       `not json`,
		"truncated":     `{"identity":"id","created":"2026-01-01T12:00:00Z"`,
		"unknown field": `{"identity":"id","created":"2026-01-01T12:00:00Z","last_used":"2026-01-01T12:00:00Z","x":1}`,
		"trailing data": `{"identity":"id","created":"2026-01-01T12:00:00Z","last_used":"2026-01-01T12:00:00Z"}{}`,
		"no identity":   `{"created":"2026-01-01T12:00:00Z","last_used":"2026-01-01T12:00:00Z"}`,
		"no created":    `{"identity":"id","last_used":"2026-01-01T12:00:00Z"}`,
		"used first":    `{"identity":"id","created":"2026-01-01T12:00:00Z","last_used":"2026-01-01T11:00:00Z"}`,
		"future":        `{"identity":"id","created":"2026-01-01T12:00:00Z","last_used":"2026-01-01T13:00:00Z"}`,
		"dup key":       `{"identity":"id","identity":"id","created":"2026-01-01T12:00:00Z","last_used":"2026-01-01T12:00:00Z"}`,
		"wrong type":    `{"identity":1,"created":"2026-01-01T12:00:00Z","last_used":"2026-01-01T12:00:00Z"}`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			s, _ := newStore(t)
			if err := os.MkdirAll(filepath.Dir(s.Path), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := fsutil.WriteFileAtomic(s.Path, []byte(body)); err != nil {
				t.Fatal(err)
			}
			if _, err := s.Load(); !errors.Is(err, ErrNoSession) {
				t.Fatalf("got %v", err)
			}
			assertGone(t, s.Path)
		})
	}
}

func TestLoadTrailingWhitespaceOK(t *testing.T) {
	s, _ := newStore(t)
	if err := os.MkdirAll(filepath.Dir(s.Path), 0o700); err != nil {
		t.Fatal(err)
	}
	body := `{"identity":"id","created":"2026-01-01T12:00:00Z","last_used":"2026-01-01T12:00:00Z"}` + "\n"
	if err := fsutil.WriteFileAtomic(s.Path, []byte(body)); err != nil {
		t.Fatal(err)
	}
	if id, err := s.Load(); err != nil || id != "id" {
		t.Fatalf("got %q %v", id, err)
	}
}

func TestRefreshSkipsDeletedSession(t *testing.T) {
	s, clock := newStore(t)
	if err := s.Save("AGE-SECRET-KEY-1TEST"); err != nil {
		t.Fatal(err)
	}
	r, raw, err := s.read(clock.t)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Delete(); err != nil {
		t.Fatal(err)
	}
	s.refresh(r, raw)
	if _, err := os.Stat(s.Path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("session came back: %v", err)
	}
}

func TestRefreshSkipsReplacedSession(t *testing.T) {
	s, clock := newStore(t)
	if err := s.Save("AGE-SECRET-KEY-1OLD"); err != nil {
		t.Fatal(err)
	}
	r, raw, err := s.read(clock.t)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Save("AGE-SECRET-KEY-1NEW"); err != nil {
		t.Fatal(err)
	}
	r.LastUsed = clock.t
	s.refresh(r, raw)
	if got := readRecord(t, s.Path).Identity; got != "AGE-SECRET-KEY-1NEW" {
		t.Fatalf("identity = %q", got)
	}
}
