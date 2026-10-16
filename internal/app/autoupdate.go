package app

import (
	"errors"
	"strings"

	"github.com/elliot40404/creds/internal/fsutil"
	"github.com/elliot40404/creds/internal/gitsync"
)

func (s *Service) InstallUpdate(tag string, install func() error) error {
	err := fsutil.WithFileLock(s.Paths.UpdateLock(), install)
	if errors.Is(err, fsutil.ErrBusy) {
		return nil
	}
	return errors.Join(err, s.recordInstall(tag, err))
}

func (s *Service) recordInstall(tag string, err error) error {
	_, uerr := gitsync.UpdateState(s.Paths.State(), func(st *gitsync.State) (bool, error) {
		st.UpdateError, st.UpdateInstalled = "", ""
		if err != nil {
			st.UpdateError, _, _ = strings.Cut(err.Error(), "\n")
		} else {
			st.UpdateInstalled = strings.TrimPrefix(tag, "v")
		}
		return true, nil
	})
	return uerr
}
