package session

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/elliot40404/creds/internal/fsutil"
)

func FuzzLoad(f *testing.F) {
	for _, s := range []string{
		"",
		"{}",
		"null",
		`{"identity":"AGE-SECRET-KEY-1","created":"2026-01-01T11:59:00Z","last_used":"2026-01-01T11:59:30Z"}`,
		`{"identity":"x","created":"2026-01-01T11:00:00Z","last_used":"2026-01-01T10:00:00Z"}`,
		`{"identity":"x","created":"9999-12-31T23:59:59Z","last_used":"9999-12-31T23:59:59Z"}`,
		`{"identity":"x","created":"0001-01-01T00:00:00Z","last_used":"2026-01-01T11:59:30Z"}`,
		`{"identity":"../\u0000","created":"2026-01-01T11:59:00Z","last_used":"2026-01-01T11:59:00Z","extra":1}`,
		`{"identity":"x","created":"2026-01-01T11:59:00+14:00","last_used":"2026-01-01T11:59:00-12:00"}`,
		"\xff\xfe",
	} {
		f.Add([]byte(s))
	}
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	path := filepath.Join(f.TempDir(), "session")
	s := &Store{Path: path, Idle: 15 * time.Minute, Hard: 4 * time.Hour, Now: func() time.Time { return now }}
	f.Fuzz(func(t *testing.T, data []byte) {
		if err := fsutil.WriteFileAtomic(path, data); err != nil {
			t.Fatal(err)
		}
		id, err := s.Load()
		if err != nil {
			if !errors.Is(err, ErrNoSession) && !errors.Is(err, ErrExpired) {
				t.Fatalf("unexpected error: %v", err)
			}
			if _, statErr := os.Stat(path); !errors.Is(statErr, os.ErrNotExist) {
				t.Fatalf("bad session not removed: %v", statErr)
			}
			return
		}
		if id == "" {
			t.Fatal("empty identity accepted")
		}
		again, err := s.Load()
		if err != nil || again != id {
			t.Fatalf("reload: %q %v", again, err)
		}
	})
}
