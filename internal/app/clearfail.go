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

func (s *Service) TakeClearFailure() string {
	st, err := gitsync.LoadState(s.Paths.State())
	if err != nil || st.ClearError == "" {
		return ""
	}
	var msg string
	_, err = gitsync.UpdateState(s.Paths.State(), func(st *gitsync.State) (bool, error) {
		msg, st.ClearError = st.ClearError, ""
		return msg != "", nil
	})
	if err != nil {
		return ""
	}
	return msg
}
