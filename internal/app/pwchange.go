package app

import "github.com/elliot40404/creds/internal/gitsync"

const passwordChanged = "sync changed the master password file. If nobody ran creds passwd on another device, the remote may have put back an old one: run creds passwd"

func (s *Service) checkPasswordChange() {
	st, err := gitsync.LoadState(s.Paths.State())
	if err == nil && !st.PasswordChanged.IsZero() {
		s.warn(passwordChanged)
	}
}

func (s *Service) ackPasswordChange() {
	_ = gitsync.AckPasswordChange(s.Paths.State(), s.Paths.SyncLock(), s.now())
}
