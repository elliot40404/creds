package app

import (
	"errors"
	"fmt"
	"io"

	"github.com/elliot40404/creds/internal/envfile"
	"github.com/elliot40404/creds/internal/vault"
)

var ErrNotEnv = errors.New("entry is not an env entry")

func (s *Service) envVars(path string) ([]envfile.Var, error) {
	v, err := s.open()
	if err != nil {
		return nil, err
	}
	defer v.Close()
	return entryVars(v, path)
}

func entryVars(v *vault.Vault, path string) ([]envfile.Var, error) {
	e, err := v.Get(path)
	if err != nil {
		return nil, err
	}
	if e.Type != vault.TypeEnv {
		return nil, fmt.Errorf("%w: %s", ErrNotEnv, path)
	}
	vars := make([]envfile.Var, len(e.Fields))
	for i, f := range e.Fields {
		vars[i] = envfile.Var{Key: f.Name, Value: f.Value}
	}
	return vars, nil
}

func (s *Service) ImportEnv(r io.Reader, path string) ([]string, error) {
	vars, warns, err := envfile.Parse(r)
	if err != nil {
		return nil, err
	}
	e := vault.Entry{Path: path, Type: vault.TypeEnv, Fields: make([]vault.Field, len(vars))}
	for i, v := range vars {
		e.Fields[i] = vault.Field{Name: v.Key, Value: v.Value, Secret: true}
	}
	if _, err := s.Add(e); err != nil {
		return nil, err
	}
	return warns, nil
}
