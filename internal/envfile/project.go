package envfile

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/elliot40404/creds/internal/safetext"
)

const ProjectFile = ".creds.toml"

var (
	ErrNoProject    = errors.New("no " + ProjectFile + " in directory")
	ErrBadRef       = errors.New("invalid ref, want path:field or path|format")
	ErrEmptyProject = errors.New("project needs env or map")
)

type Ref struct {
	Path   string
	Field  string
	Format string
}

type Project struct {
	Env  string
	Map  map[string]Ref
	File string
	Sum  string
}

type projectFile struct {
	Env *string           `toml:"env"`
	Map map[string]string `toml:"map"`
}

func ParseRef(s string) (Ref, error) {
	i := strings.LastIndexAny(s, ":|")
	if i < 0 {
		return Ref{}, fmt.Errorf("%w: %q", ErrBadRef, s)
	}
	path, sel := s[:i], s[i+1:]
	if !cleanPart(path) || !cleanPart(sel) || strings.ContainsAny(path, ":|") {
		return Ref{}, fmt.Errorf("%w: %q", ErrBadRef, s)
	}
	if s[i] == ':' {
		return Ref{Path: path, Field: sel}, nil
	}
	return Ref{Path: path, Format: sel}, nil
}

func cleanPart(s string) bool {
	return s != "" && strings.TrimSpace(s) == s && !safetext.HasControl(s)
}

func LoadProject(dir string) (Project, error) {
	path, err := filepath.Abs(filepath.Join(dir, ProjectFile))
	if err != nil {
		return Project{}, err
	}
	f, err := os.Open(filepath.Clean(path))
	if errors.Is(err, os.ErrNotExist) {
		return Project{}, ErrNoProject
	}
	if err != nil {
		return Project{}, err
	}
	defer func() { _ = f.Close() }()
	src, err := readAll(f)
	if err != nil {
		return Project{}, fmt.Errorf("project %s: %w", path, err)
	}
	p, err := decodeProject(src)
	if err != nil {
		return Project{}, fmt.Errorf("project %s: %w", path, err)
	}
	p.File, p.Sum = path, sum(src)
	return p, nil
}

func decodeProject(src string) (Project, error) {
	var pf projectFile
	md, err := toml.Decode(src, &pf)
	if err != nil {
		return Project{}, err
	}
	if keys := md.Undecoded(); len(keys) > 0 {
		return Project{}, fmt.Errorf("unknown key %s", keys[0])
	}
	if pf.Env == nil && len(pf.Map) == 0 {
		return Project{}, ErrEmptyProject
	}
	p := Project{Map: make(map[string]Ref, len(pf.Map))}
	if pf.Env != nil {
		if !cleanPart(*pf.Env) {
			return Project{}, fmt.Errorf("env: invalid path %q", *pf.Env)
		}
		p.Env = *pf.Env
	}
	for k, v := range pf.Map {
		if !validKey(k) {
			return Project{}, fmt.Errorf("map: %w %q", ErrBadKey, k)
		}
		ref, err := ParseRef(v)
		if err != nil {
			return Project{}, fmt.Errorf("map.%s: %w", k, err)
		}
		p.Map[k] = ref
	}
	return p, nil
}
