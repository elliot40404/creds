package cli

import (
	"os"
	"path/filepath"

	"github.com/elliot40404/creds/internal/app"
	"github.com/elliot40404/creds/internal/editor"
	"github.com/elliot40404/creds/internal/fsutil"
	"github.com/spf13/cobra"
)

func configEditCmd(env Env) *cobra.Command {
	return &cobra.Command{
		Use:   "edit",
		Short: "Open the settings file in your editor",
		Args:  cobra.NoArgs,
		RunE: env.run(func(s *app.Service, _ []string) error {
			return env.editConfig(s)
		}),
	}
}

func (e Env) Edit(path string) error {
	if e.Editor != nil {
		return e.Editor(path)
	}
	return editor.Open(path, e.In, e.Err)
}

func (e Env) editConfig(s *app.Service) error {
	before, err := s.ConfigText()
	if err != nil {
		return err
	}
	dir, err := os.MkdirTemp("", "creds-config")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(dir) }()
	path := filepath.Join(dir, "config.toml")
	if err := fsutil.WriteFileAtomic(path, before); err != nil {
		return err
	}
	if err := e.Edit(path); err != nil {
		return err
	}
	after, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return err
	}
	if string(after) == string(before) {
		e.note("settings unchanged")
		return nil
	}
	if err := s.SaveConfigText(after); err != nil {
		return err
	}
	e.note("settings saved")
	return nil
}
