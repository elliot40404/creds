package app

import (
	"strings"

	"github.com/elliot40404/creds/internal/gitsync"
)

func (s *Service) RecordClearFailure(err error) error {
	msg, _, _ := strings.Cut(err.Error(), "\n")
	_, uerr := gitsync.UpdateState(s.Paths.State(), func(st *gitsync.State) (bool, error) {
		st.ClearError = msg
		return true, nil
	})
	return uerr
}

func (s *Service) takeOnce(n *Notice) {
	var was gitsync.State
	_, err := gitsync.UpdateState(s.Paths.State(), func(st *gitsync.State) (bool, error) {
		was = *st
		st.ClearError, st.UpdateError, st.UpdateInstalled = "", "", ""
		return hasOnce(was), nil
	})
	if err != nil {
		return
	}
	n.ClearFailure, n.UpdateFailed, n.Updated = was.ClearError, was.UpdateError != "", was.UpdateInstalled
}

func hasOnce(st gitsync.State) bool {
	return st.ClearError != "" || st.UpdateError != "" || st.UpdateInstalled != ""
}
