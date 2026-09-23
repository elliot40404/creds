package app

import (
	"testing"
	"time"

	"github.com/elliot40404/creds/internal/config"
	"github.com/elliot40404/creds/internal/gitsync"
	"github.com/elliot40404/creds/internal/search"
)

type spawnCounter struct{ n int }

func (c *spawnCounter) spawn() error {
	c.n++
	return nil
}

func spawning(s *Service) *spawnCounter {
	c := &spawnCounter{}
	s.Spawn = c.spawn
	return c
}

func (s *Service) fresh() *Service {
	return &Service{Paths: s.Paths, Config: s.Config, Prompter: s.Prompter, Now: s.Now, LogN: s.LogN, Spawn: s.Spawn}
}

func TestWriteSpawnsOnlyWithRemote(t *testing.T) {
	t.Parallel()
	s, _, _, _ := initVault(t)
	sc := spawning(s)
	if _, err := s.Add(dbEntry("a")); err != nil {
		t.Fatal(err)
	}
	if sc.n != 0 {
		t.Fatalf("spawned without remote: %d", sc.n)
	}
	withRemote(t, s)
	if _, err := s.Add(dbEntry("b")); err != nil {
		t.Fatal(err)
	}
	if err := s.Delete("a"); err != nil {
		t.Fatal(err)
	}
	if sc.n != 1 {
		t.Fatalf("spawns per command = %d", sc.n)
	}
	if _, err := s.List(); err != nil {
		t.Fatal(err)
	}
	if sc.n != 1 {
		t.Fatalf("read after write spawned: %d", sc.n)
	}
}

func TestReadSpawnsOnlyWhenStale(t *testing.T) {
	t.Parallel()
	s, _, c, _ := initVault(t)
	sc := spawning(s)
	if _, err := s.List(); err != nil {
		t.Fatal(err)
	}
	if sc.n != 0 {
		t.Fatal("spawned without remote")
	}
	withRemote(t, s)
	s = s.fresh()
	if _, err := s.List(); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Search("x", search.SortPath); err != nil {
		t.Fatal(err)
	}
	if sc.n != 1 {
		t.Fatalf("never synced: spawns = %d", sc.n)
	}
	if err := gitsync.SaveState(s.Paths.State(), gitsync.State{LastSync: c.t.Add(-time.Minute)}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.fresh().List(); err != nil {
		t.Fatal(err)
	}
	if sc.n != 1 {
		t.Fatalf("fresh state: spawns = %d", sc.n)
	}
	c.t = c.t.Add(s.Config.Sync.Stale)
	lock, err := gitsync.AcquireLock(s.Paths.SyncLock(), c.t)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.fresh().List(); err != nil {
		t.Fatal(err)
	}
	if sc.n != 1 {
		t.Fatalf("lock held: spawns = %d", sc.n)
	}
	if err := lock.Release(); err != nil {
		t.Fatal(err)
	}
	if _, err := s.fresh().Get("missing"); err == nil {
		t.Fatal("want not found")
	}
	if sc.n != 2 {
		t.Fatalf("stale: spawns = %d", sc.n)
	}
}

func TestSyncDue(t *testing.T) {
	t.Parallel()
	s, _, c, _ := initVault(t)
	if s.SyncDue() {
		t.Fatal("due without remote")
	}
	withRemote(t, s)
	if !s.SyncDue() {
		t.Fatal("not due after never syncing")
	}
	if err := gitsync.SaveState(s.Paths.State(), gitsync.State{LastSync: c.t.Add(-time.Minute)}); err != nil {
		t.Fatal(err)
	}
	if s.SyncDue() {
		t.Fatal("due one minute after a sync")
	}
	if err := gitsync.SaveState(s.Paths.State(), gitsync.State{LastSync: c.t.Add(-time.Hour)}); err != nil {
		t.Fatal(err)
	}
	if !s.SyncDue() {
		t.Fatal("not due an hour after a sync")
	}
}

func TestShouldSpawn(t *testing.T) {
	t.Parallel()
	at := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	cfg := config.Sync{Stale: 5 * time.Minute, Every: 30 * time.Second, After: 2 * time.Second}
	cases := map[string]struct {
		st   gitsync.State
		now  time.Time
		want bool
	}{
		"never spawned":      {gitsync.State{}, at, true},
		"just spawned":       {gitsync.State{LastSpawn: at}, at, false},
		"inside every":       {gitsync.State{LastSpawn: at}, at.Add(29 * time.Second), false},
		"on every":           {gitsync.State{LastSpawn: at}, at.Add(30 * time.Second), true},
		"past every":         {gitsync.State{LastSpawn: at}, at.Add(time.Minute), true},
		"after a failure":    {gitsync.State{LastSpawn: at, LastResult: "error"}, at.Add(time.Minute), false},
		"failure backed off": {gitsync.State{LastSpawn: at, LastResult: "error"}, at.Add(150 * time.Second), true},
		"backoff capped": {
			gitsync.State{LastSpawn: at, LastResult: "error"},
			at.Add(5 * time.Minute),
			true,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if got := shouldSpawn(tc.st, tc.now, cfg); got != tc.want {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestBackoffIsCappedByStale(t *testing.T) {
	t.Parallel()
	at := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	cfg := config.Sync{Stale: time.Minute, Every: 30 * time.Second}
	st := gitsync.State{LastSpawn: at, LastResult: "error"}
	if shouldSpawn(st, at.Add(59*time.Second), cfg) {
		t.Fatal("spawned before the capped backoff")
	}
	if !shouldSpawn(st, at.Add(time.Minute), cfg) {
		t.Fatal("backoff not capped at sync.stale")
	}
}

func TestWriteSpawnsPerThrottle(t *testing.T) {
	t.Parallel()
	s, _, c, _ := initVault(t)
	sc := spawning(s)
	withRemote(t, s)
	for _, path := range []string{"a", "b", "c"} {
		if _, err := s.Add(dbEntry(path)); err != nil {
			t.Fatal(err)
		}
	}
	if sc.n != 1 {
		t.Fatalf("three adds inside sync.every spawned %d times, want 1", sc.n)
	}
	c.t = c.t.Add(s.Config.Sync.Every)
	if _, err := s.Add(dbEntry("d")); err != nil {
		t.Fatal(err)
	}
	if sc.n != 2 {
		t.Fatalf("after sync.every: spawns = %d, want 2", sc.n)
	}
}

func TestWriteMarksPendingUntilASyncSucceeds(t *testing.T) {
	t.Parallel()
	s, _, c, _ := initVault(t)
	spawning(s)
	withRemote(t, s)
	if _, err := s.Add(dbEntry("a")); err != nil {
		t.Fatal(err)
	}
	st, err := gitsync.LoadState(s.Paths.State())
	if err != nil {
		t.Fatal(err)
	}
	if st.PendingSince.IsZero() {
		t.Fatal("write left nothing pending")
	}
	first := st.PendingSince
	c.t = c.t.Add(time.Minute)
	if _, err := s.Add(dbEntry("b")); err != nil {
		t.Fatal(err)
	}
	if st, _ = gitsync.LoadState(s.Paths.State()); !st.PendingSince.Equal(first) {
		t.Fatalf("pending moved to %v, want the first write at %v", st.PendingSince, first)
	}
}
