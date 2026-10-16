package app

import (
	"strings"
	"time"

	"github.com/elliot40404/creds/internal/config"
	"github.com/elliot40404/creds/internal/gitsync"
	"github.com/elliot40404/creds/internal/update"
)

type Notice struct {
	SyncFailure  string
	ClearFailure string
	Update       string
	CheckUpdate  bool
	UpdateFailed bool
	Updated      string
}

func (s *Service) Notice() Notice {
	st, err := gitsync.LoadState(s.Paths.State())
	if err != nil {
		return Notice{}
	}
	mode := s.CurrentConfig().Update.Current()
	n := Notice{
		SyncFailure: s.syncFailure(st),
		Update:      newRelease(mode, s.Release, st.UpdateLatest),
		CheckUpdate: checkDue(mode, s.Release, st.UpdateChecked, s.now()),
	}
	if hasOnce(st) {
		s.takeOnce(&n)
	}
	return n
}

func newRelease(mode config.UpdateMode, release, latest string) string {
	if mode == config.UpdateOff || !update.Newer(latest, release) {
		return ""
	}
	return strings.TrimPrefix(latest, "v")
}

func checkDue(mode config.UpdateMode, release string, checked, now time.Time) bool {
	if _, ok := update.Parse(release); !ok || mode == config.UpdateOff {
		return false
	}
	return update.Due(checked, now)
}
