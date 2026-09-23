package app

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/elliot40404/creds/internal/config"
	"github.com/elliot40404/creds/internal/gitsync"
)

var remoteSection = []byte(`[remote "` + gitsync.Remote + `"]`)

const failureBackoff = 5

func shouldSpawn(st gitsync.State, now time.Time, cfg config.Sync) bool {
	if st.LastSpawn.IsZero() {
		return true
	}
	wait := cfg.Every
	if st.LastResult == "error" {
		wait = min(cfg.Every*failureBackoff, cfg.Stale)
	}
	return !now.Before(st.LastSpawn.Add(wait))
}

func (s *Service) afterWrite() {
	if s.Spawn == nil || !s.hasRemote() {
		return
	}
	now := s.now()
	st, err := gitsync.MarkPending(s.Paths.State(), now)
	if err != nil || !shouldSpawn(st, now, s.CurrentConfig().Sync) {
		return
	}
	s.spawn(now)
}

func (s *Service) afterRead() {
	if s.Spawn == nil || !s.firstTime(&s.readChecked) {
		return
	}
	st, err := gitsync.LoadState(s.Paths.State())
	if err != nil {
		return
	}
	now := s.now()
	if !s.syncDue(st, now) || !shouldSpawn(st, now, s.CurrentConfig().Sync) {
		return
	}
	s.spawn(now)
}

func (s *Service) SyncDue() bool {
	st, err := gitsync.LoadState(s.Paths.State())
	return err == nil && s.syncDue(st, s.now())
}

func (s *Service) syncDue(st gitsync.State, now time.Time) bool {
	return now.Sub(st.LastSync) >= s.CurrentConfig().Sync.Stale && !gitsync.Locked(s.Paths.SyncLock(), now) && s.hasRemote()
}

func (s *Service) spawn(now time.Time) {
	_ = gitsync.MarkSpawn(s.Paths.State(), now)
	_ = s.Spawn()
}

func (s *Service) hasRemote() bool {
	data, err := os.ReadFile(s.vaultFile(filepath.Join(gitDir, "config")))
	return err == nil && bytes.Contains(data, remoteSection)
}

func (s *Service) SyncFailure() string {
	st, err := gitsync.LoadState(s.Paths.State())
	if err != nil || st.LastResult != "error" || !s.hasRemote() {
		return ""
	}
	line, _, _ := strings.Cut(st.LastError, "\n")
	return line
}
