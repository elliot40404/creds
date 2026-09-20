package app

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/elliot40404/creds/internal/config"
)

var ErrStrayHome = errors.New("creds home has settings files this vault did not create")

func (s *Service) homeFiles() []string {
	return []string{s.Paths.Config(), s.Paths.Trust(), s.Paths.State()}
}

func (s *Service) strayFiles() []string {
	var found []string
	for _, p := range s.homeFiles() {
		if _, err := os.Lstat(p); err == nil {
			found = append(found, filepath.Base(p))
		}
	}
	return found
}

func (s *Service) confirmStray() error {
	if !s.firstTime(&s.strayChecked) {
		return nil
	}
	stray := s.strayFiles()
	if len(stray) == 0 {
		return nil
	}
	ok, err := s.Prompter.Confirm(fmt.Sprintf("%s already holds %s but has no vault, they may not be yours.%s Keep them?", s.Paths.Home, strings.Join(stray, ", "), s.configSetNote(stray)))
	if err != nil {
		return err
	}
	if !ok {
		return s.removeStray()
	}
	s.warn(fmt.Sprintf("kept %s found in %s before setup", strings.Join(stray, ", "), s.Paths.Home))
	return nil
}

func (s *Service) configSetNote(stray []string) string {
	if !slices.Contains(stray, filepath.Base(s.Paths.Config())) {
		return ""
	}
	return " creds config set writes config.toml even before init, so keep them if you ran it."
}

func (s *Service) removeStray() error {
	for _, p := range s.homeFiles() {
		if err := os.Remove(p); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
	}
	s.setConfig(config.Default())
	return nil
}
