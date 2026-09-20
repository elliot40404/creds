package session

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/elliot40404/creds/internal/fsutil"
)

func TestLeftReportsSmallestRemaining(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	s := &Store{Path: filepath.Join(t.TempDir(), "session"), Idle: time.Hour, Hard: 4 * time.Hour, Now: func() time.Time { return now }}
	if _, err := s.Left(); !errors.Is(err, ErrNoSession) {
		t.Fatalf("err %v", err)
	}
	if err := s.Save("id"); err != nil {
		t.Fatal(err)
	}
	now = now.Add(30 * time.Minute)
	left, err := s.Left()
	if err != nil || left != 30*time.Minute {
		t.Fatalf("left %v err %v", left, err)
	}
	now = now.Add(3*time.Hour + 45*time.Minute)
	if _, err := s.Left(); !errors.Is(err, ErrExpired) {
		t.Fatalf("err %v", err)
	}
}

func TestLeftDoesNotRefreshTheSession(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	s := &Store{Path: filepath.Join(t.TempDir(), "session"), Idle: time.Hour, Hard: 4 * time.Hour, Now: func() time.Time { return now }}
	if err := s.Save("id"); err != nil {
		t.Fatal(err)
	}
	now = now.Add(30 * time.Minute)
	if _, err := s.Left(); err != nil {
		t.Fatal(err)
	}
	now = now.Add(31 * time.Minute)
	if _, err := s.Load(); !errors.Is(err, ErrExpired) {
		t.Fatalf("err %v", err)
	}
}

func TestLeftRemovesAnExpiredSession(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	path := filepath.Join(t.TempDir(), "session")
	s := &Store{Path: path, Idle: time.Hour, Hard: 4 * time.Hour, Now: func() time.Time { return now }}
	if err := s.Save("id"); err != nil {
		t.Fatal(err)
	}
	now = now.Add(2 * time.Hour)
	if _, err := s.Left(); !errors.Is(err, ErrExpired) {
		t.Fatalf("err %v", err)
	}
	if _, err := os.Stat(path); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("session file left behind: %v", err)
	}
}

func TestLeftRemovesACorruptSession(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	path := filepath.Join(t.TempDir(), "session")
	if err := fsutil.WriteFileAtomic(path, []byte("not a session")); err != nil {
		t.Fatal(err)
	}
	s := &Store{Path: path, Idle: time.Hour, Hard: 4 * time.Hour, Now: func() time.Time { return now }}
	if _, err := s.Left(); !errors.Is(err, ErrNoSession) {
		t.Fatalf("err %v", err)
	}
	if _, err := os.Stat(path); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("session file left behind: %v", err)
	}
}
