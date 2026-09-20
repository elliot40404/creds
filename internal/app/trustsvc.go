package app

import (
	"fmt"

	"github.com/elliot40404/creds/internal/envfile"
)

func (s *Service) TrustList() ([]string, error) {
	return envfile.TrustList(s.Paths.Trust())
}

func (s *Service) Untrust(file string) error {
	found, err := envfile.Untrust(s.Paths.Trust(), file)
	if err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("%w: %s", ErrUntrustedProject, file)
	}
	return nil
}
