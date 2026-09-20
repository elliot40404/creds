package config

import (
	"path/filepath"

	"github.com/elliot40404/creds/internal/fsutil"
)

func Save(path string, c Config) error {
	data, err := Encode(c)
	if err != nil {
		return err
	}
	if target, err := filepath.EvalSymlinks(path); err == nil {
		path = target
	}
	return fsutil.WriteFileAtomic(path, data)
}
