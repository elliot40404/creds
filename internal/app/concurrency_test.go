package app

import (
	"sync"
	"testing"
)

func TestServiceIsSafeAcrossGoroutines(t *testing.T) {
	t.Parallel()
	s, _, _, _ := initVault(t)
	if _, err := s.Add(dbEntry("work/db")); err != nil {
		t.Fatal(err)
	}
	withRemote(t, s)
	cfg := s.Config
	var wg sync.WaitGroup
	for range 2 {
		wg.Go(func() { _, _ = s.Sync() })
		wg.Go(func() { _, _ = s.List() })
		wg.Go(func() { _ = s.saveConfig(cfg) })
		wg.Go(func() { _ = s.Warnings() })
		wg.Go(func() { _ = s.ConfigFields() })
	}
	wg.Wait()
}
