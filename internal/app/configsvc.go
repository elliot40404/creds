package app

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/elliot40404/creds/internal/config"
	"github.com/elliot40404/creds/internal/fsutil"
	"github.com/elliot40404/creds/internal/gitsync"
	"github.com/elliot40404/creds/internal/render"
)

type ConfigField struct {
	Key     string   `json:"key"`
	Value   string   `json:"value"`
	Default string   `json:"default,omitzero"`
	Doc     string   `json:"doc,omitzero"`
	Choices []string `json:"choices,omitempty"`
	Derived string   `json:"derived,omitzero"`
	Source  string   `json:"source,omitzero"`
}

const (
	SourceGit      = "git global"
	SourceHostname = "hostname"
	sourceBuiltIn  = "built in"
)

func (s *Service) ConfigFields() []ConfigField {
	c := s.CurrentConfig()
	fields := config.Fields(c)
	out := make([]ConfigField, len(fields))
	for i, f := range fields {
		out[i] = ConfigField{Key: f.Key, Value: f.Value(c), Default: f.Default(), Doc: f.Doc, Choices: f.Choices}
		if out[i].Value == "" {
			out[i].Derived, out[i].Source = s.derived(f.Key)
		}
	}
	return out
}

func (s *Service) derived(key string) (value, source string) {
	switch key {
	case "sync.name":
		c := gitsync.LookupCommitter()
		return c.Name, gitSource(c.NameGlobal)
	case "sync.email":
		c := gitsync.LookupCommitter()
		return c.Email, gitSource(c.EmailGlobal)
	case "vault.machine":
		return config.DefaultMachine(), SourceHostname
	}
	return "", ""
}

func gitSource(global bool) string {
	if global {
		return SourceGit
	}
	return sourceBuiltIn
}

func (s *Service) ConfigGet(key string) (string, error) {
	return config.Get(s.CurrentConfig(), key)
}

func (s *Service) ConfigSet(key, value string) error {
	c, err := config.Set(s.CurrentConfig(), key, value)
	if err != nil {
		return err
	}
	return s.saveConfig(c)
}

func (s *Service) ConfigUnset(key string) error {
	c, err := config.Unset(s.CurrentConfig(), key)
	if err != nil {
		return err
	}
	return s.saveConfig(c)
}

func (s *Service) saveConfig(c config.Config) error {
	if err := fsutil.EnsureDir(s.Paths.Home); err != nil {
		return err
	}
	if err := config.Save(s.Paths.Config(), c); err != nil {
		return err
	}
	s.setConfig(c)
	return nil
}

func (s *Service) ConfigText() ([]byte, error) {
	data, err := os.ReadFile(filepath.Clean(s.Paths.Config()))
	if errors.Is(err, fs.ErrNotExist) {
		return config.Encode(s.CurrentConfig())
	}
	if err != nil {
		return nil, err
	}
	return data, nil
}

func (s *Service) SaveConfigText(data []byte) error {
	c, err := config.Decode(data)
	if err != nil {
		return err
	}
	return s.saveConfig(c)
}

func (s *Service) ConfigPreview(key, value string) (string, error) {
	name, ok := strings.CutPrefix(key, config.FormatPrefix)
	if !ok {
		return "", fmt.Errorf("%w: %s", config.ErrUnknownKey, key)
	}
	engine, format, ok := strings.Cut(name, ".")
	if !ok {
		return "", fmt.Errorf("%w: %s", config.ErrUnknownKey, key)
	}
	return render.Preview(engine, format, value, s.CurrentConfig().Render.Shell)
}

func (s *Service) SyncAfter() time.Duration {
	return s.CurrentConfig().Sync.After
}
