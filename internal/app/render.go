package app

import (
	"errors"
	"fmt"
	"maps"

	"github.com/elliot40404/creds/internal/render"
	"github.com/elliot40404/creds/internal/vault"
)

var ErrNotDatabase = errors.New("entry is not a database entry")

func (s *Service) Formats(path string) ([]string, error) {
	engine, _, err := s.renderTarget(path)
	if err != nil {
		return nil, err
	}
	r, err := s.renderer("")
	if err != nil {
		return nil, err
	}
	return r.Formats(engine)
}

func (s *Service) Render(path, format string, sh render.Shell) (string, error) {
	engine, c, err := s.renderTarget(path)
	if err != nil {
		return "", err
	}
	r, err := s.renderer(sh)
	if err != nil {
		return "", err
	}
	return r.Render(engine, format, c)
}

func (s *Service) ShellVaries(path, format string) (bool, error) {
	engine, c, err := s.renderTarget(path)
	if err != nil {
		return false, err
	}
	var out [2]string
	for i, sh := range []render.Shell{render.Bash, render.Pwsh} {
		r, err := s.renderer(sh)
		if err != nil {
			return false, err
		}
		if out[i], err = r.Render(engine, format, c); err != nil {
			return false, err
		}
	}
	return out[0] != out[1], nil
}

func (s *Service) RenderShell() render.Shell {
	if sh := s.CurrentConfig().Render.Shell; sh != "" {
		return sh
	}
	return render.DefaultShell()
}

func (s *Service) renderer(sh render.Shell) (*render.Renderer, error) {
	c := s.CurrentConfig().Render
	if sh == "" {
		sh = c.Shell
	}
	return render.New(c.Formats, sh)
}

func (s *Service) renderTarget(path string) (string, render.Conn, error) {
	e, err := s.Get(path)
	if err != nil {
		return "", render.Conn{}, err
	}
	return databaseConn(e)
}

func databaseConn(e vault.Entry) (string, render.Conn, error) {
	if e.Type != vault.TypeDatabase {
		return "", render.Conn{}, fmt.Errorf("%w: %s", ErrNotDatabase, e.Path)
	}
	c := render.Conn{
		Scheme:   fieldValue(e, fieldScheme),
		Host:     e.Host,
		Port:     fieldValue(e, FieldPort),
		Username: e.Username,
		Password: fieldValue(e, FieldPassword),
		Database: fieldValue(e, FieldDatabase),
		Params:   maps.Clone(e.Params),
	}
	for _, f := range e.Fields {
		if f.Secret && f.Name != FieldPassword && render.SecretParam(f.Name) {
			if c.Params == nil {
				c.Params = map[string]string{}
			}
			c.Params[f.Name] = f.Value
		}
	}
	return fieldValue(e, FieldEngine), c, nil
}

func fieldValue(e vault.Entry, name string) string {
	v, _ := Lookup(e, name)
	return v
}
