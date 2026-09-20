package app

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/elliot40404/creds/internal/envfile"
	"github.com/elliot40404/creds/internal/render"
	"github.com/elliot40404/creds/internal/vault"
)

var (
	ErrNoEnvPath        = errors.New("no path given and no " + envfile.ProjectFile + " in current directory")
	ErrUntrustedProject = errors.New("project file is not trusted")
)

type interactive interface {
	Interactive() bool
}

func (s *Service) ProjectEnv(dir, path string) ([]envfile.Var, error) {
	if path != "" {
		return s.envVars(path)
	}
	p, err := envfile.LoadProject(dir)
	if errors.Is(err, envfile.ErrNoProject) {
		return nil, ErrNoEnvPath
	}
	if err != nil {
		return nil, err
	}
	if err := s.checkTrust(p); err != nil {
		return nil, err
	}
	v, err := s.open()
	if err != nil {
		return nil, err
	}
	defer v.Close()
	var vars []envfile.Var
	if p.Env != "" {
		if vars, err = entryVars(v, p.Env); err != nil {
			return nil, fmt.Errorf("env: %w", err)
		}
	}
	res := refResolver{v: v, s: s}
	for _, key := range slices.Sorted(maps.Keys(p.Map)) {
		val, err := res.resolve(p.Map[key])
		if err != nil {
			return nil, fmt.Errorf("map.%s: %w", key, err)
		}
		vars = setVar(vars, envfile.Var{Key: key, Value: val})
	}
	return vars, nil
}

func (s *Service) TrustProject(p envfile.Project) error {
	return envfile.Trust(s.Paths.Trust(), p)
}

func (s *Service) checkTrust(p envfile.Project) error {
	ok, err := envfile.Trusted(s.Paths.Trust(), p)
	if err != nil || ok {
		return err
	}
	untrusted := fmt.Errorf("%w: %s is new or changed, review it and run creds trust", ErrUntrustedProject, p.File)
	if t, ok := s.Prompter.(interactive); ok && !t.Interactive() {
		return untrusted
	}
	if err := s.Prompter.Show("New or changed "+p.File+", it asks for", strings.Join(p.Refs(), "\n  ")); err != nil {
		return untrusted
	}
	yes, err := s.Prompter.Confirm("Trust this file and hand these secrets to the command?")
	switch {
	case err != nil:
		return untrusted
	case !yes:
		return ErrAborted
	}
	return envfile.Trust(s.Paths.Trust(), p)
}

func setVar(vars []envfile.Var, nv envfile.Var) []envfile.Var {
	i := slices.IndexFunc(vars, func(v envfile.Var) bool { return v.Key == nv.Key })
	if i < 0 {
		return append(vars, nv)
	}
	vars[i] = nv
	return vars
}

type refResolver struct {
	v        *vault.Vault
	s        *Service
	renderer *render.Renderer
}

func (r *refResolver) resolve(ref envfile.Ref) (string, error) {
	e, err := r.v.Get(ref.Path)
	if err != nil {
		return "", err
	}
	if ref.Format == "" {
		return Lookup(e, ref.Field)
	}
	engine, c, err := databaseConn(e)
	if err != nil {
		return "", err
	}
	if r.renderer == nil {
		if r.renderer, err = r.s.renderer(""); err != nil {
			return "", err
		}
	}
	return r.renderer.Render(engine, ref.Format, c)
}
