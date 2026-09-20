package app

import (
	"fmt"
)

func (s *Service) newPassword() (string, error) {
	return Retry(s.Prompter, s.askNewPassword, ErrShortPassword, ErrWeakPassword, ErrPasswordMismatch)
}

func (s *Service) askNewPassword() (string, error) {
	pw, err := s.Prompter.Password("New master password")
	if err != nil {
		return "", err
	}
	reason, err := checkPassword(pw)
	if err != nil {
		return "", err
	}
	if reason != "" {
		return "", fmt.Errorf("%w: %s", ErrWeakPassword, reason)
	}
	again, err := s.Prompter.Password("Repeat master password")
	if err != nil {
		return "", err
	}
	if again != pw {
		return "", ErrPasswordMismatch
	}
	return pw, nil
}
