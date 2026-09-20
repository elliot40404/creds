package config

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/BurntSushi/toml"
	"github.com/elliot40404/creds/internal/fsutil"
	"github.com/elliot40404/creds/internal/render"
	"github.com/elliot40404/creds/internal/safetext"
)

const maxConfigSize = 1 << 20

func Load(path string) (Config, error) {
	c := Default()
	if target, err := filepath.EvalSymlinks(path); err == nil {
		path = target
	}
	data, err := fsutil.ReadFileLimit(path, maxConfigSize)
	if errors.Is(err, fs.ErrNotExist) {
		return c, nil
	}
	if err != nil {
		return Config{}, err
	}
	c, err = Decode(data)
	if err != nil {
		return Config{}, fmt.Errorf("config %s: %w", path, err)
	}
	return c, nil
}

func Decode(data []byte) (Config, error) {
	c := Default()
	md, err := toml.Decode(string(data), &c)
	if err != nil {
		return Config{}, err
	}
	if keys := md.Undecoded(); len(keys) > 0 {
		return Config{}, fmt.Errorf("unknown keys: %s", joinKeys(keys))
	}
	if err := c.validate(); err != nil {
		return Config{}, err
	}
	return c, nil
}

func Encode(c Config) ([]byte, error) {
	if err := c.validate(); err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := toml.NewEncoder(&buf).Encode(c); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (c Config) validate() error {
	if err := checkLimits(c); err != nil {
		return err
	}
	if err := checkUI(c.UI); err != nil {
		return err
	}
	if err := checkVault(c.Vault); err != nil {
		return err
	}
	if err := checkSign(c.Sync); err != nil {
		return err
	}
	if c.Session.Idle > c.Session.Hard {
		return errors.New("session.idle must not exceed session.hard")
	}
	for name, tmpl := range c.Render.Formats {
		if safetext.HasControl(name) || !utf8.ValidString(tmpl) {
			return errors.New("render.formats: names must be plain text and templates valid UTF-8")
		}
	}
	if _, err := render.New(c.Render.Formats, c.Render.Shell); err != nil {
		key := "render.formats"
		if errors.Is(err, render.ErrUnknownShell) {
			key = "render.shell"
		}
		return fmt.Errorf("%s: %w", key, err)
	}
	return nil
}

func joinKeys(keys []toml.Key) string {
	s := make([]string, len(keys))
	for i, k := range keys {
		s[i] = k.String()
	}
	return strings.Join(s, ", ")
}
