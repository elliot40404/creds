package session

import (
	"errors"
	"io/fs"
	"os"
	"sync"
	"testing"
	"time"
)

func TestLockDuringRefreshWins(t *testing.T) {
	s, clock := newStore(t)
	if err := s.Save("AGE-SECRET-KEY-X"); err != nil {
		t.Fatal(err)
	}
	clock.advance(time.Minute)
	var wg sync.WaitGroup
	beforeRefresh = func() {
		wg.Go(func() {
			if err := s.Delete(); err != nil {
				t.Error(err)
			}
		})
		time.Sleep(50 * time.Millisecond)
	}
	defer func() { beforeRefresh = func() {} }()
	if _, err := s.Load(); err != nil {
		t.Fatal(err)
	}
	wg.Wait()
	if _, err := os.Stat(s.Path); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("session survived lock: %v", err)
	}
}

func TestLockWinsOverConcurrentLoad(t *testing.T) {
	s, _ := newStore(t)
	for i := range 200 {
		if err := s.Save("AGE-SECRET-KEY-X"); err != nil {
			t.Fatal(err)
		}
		var wg sync.WaitGroup
		wg.Go(func() { _, _ = s.Load() })
		wg.Go(func() {
			if err := s.Delete(); err != nil {
				t.Error(err)
			}
		})
		wg.Wait()
		if _, err := os.Stat(s.Path); !errors.Is(err, fs.ErrNotExist) {
			t.Fatalf("run %d: session survived lock: %v", i, err)
		}
	}
}
