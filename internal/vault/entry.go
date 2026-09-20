package vault

import (
	"cmp"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
	"time"
	"uuid"

	"github.com/elliot40404/creds/internal/safetext"
)

type Type string

const (
	TypeLogin    Type = "login"
	TypeAPI      Type = "api"
	TypeDatabase Type = "database"
	TypeSSH      Type = "ssh"
	TypeNote     Type = "note"
	TypeCommand  Type = "command"
	TypeEnv      Type = "env"
	TypeGeneric  Type = "generic"
)

var types = []Type{TypeLogin, TypeAPI, TypeDatabase, TypeSSH, TypeNote, TypeCommand, TypeEnv, TypeGeneric}

var (
	ErrInvalidEntry = errors.New("invalid entry")
	ErrBadPath      = fmt.Errorf("%w: bad path", ErrInvalidEntry)
)

type Field struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Secret bool   `json:"secret,omitzero"`
}

type Entry struct {
	ID       uuid.UUID         `json:"id"`
	Path     string            `json:"path"`
	Name     string            `json:"name,omitzero"`
	Type     Type              `json:"type"`
	Username string            `json:"username,omitzero"`
	Host     string            `json:"host,omitzero"`
	URL      string            `json:"url,omitzero"`
	Notes    string            `json:"notes,omitzero"`
	Tags     []string          `json:"tags,omitzero"`
	Fields   []Field           `json:"fields,omitzero"`
	Params   map[string]string `json:"params,omitzero"`
	Created  time.Time         `json:"created"`
	Updated  time.Time         `json:"updated"`
	Machine  string            `json:"machine,omitzero"`
	History  []Revision        `json:"history,omitzero"`
}

type Revision struct {
	Machine  string            `json:"machine,omitzero"`
	At       time.Time         `json:"at"`
	Name     string            `json:"name,omitzero"`
	Username string            `json:"username,omitzero"`
	Host     string            `json:"host,omitzero"`
	URL      string            `json:"url,omitzero"`
	Notes    string            `json:"notes,omitzero"`
	Tags     []string          `json:"tags,omitzero"`
	Fields   []Field           `json:"fields,omitzero"`
	Params   map[string]string `json:"params,omitzero"`
}

func (e Entry) revision() Revision {
	return Revision{
		Machine:  e.Machine,
		At:       e.Updated,
		Name:     e.Name,
		Username: e.Username,
		Host:     e.Host,
		URL:      e.URL,
		Notes:    e.Notes,
		Tags:     slices.Clone(e.Tags),
		Fields:   slices.Clone(e.Fields),
		Params:   maps.Clone(e.Params),
	}
}

func (r Revision) Clone() Revision {
	r.Tags = slices.Clone(r.Tags)
	r.Fields = slices.Clone(r.Fields)
	r.Params = maps.Clone(r.Params)
	return r
}

func sameValues(a, b Revision) bool {
	return a.Name == b.Name && a.Username == b.Username && a.Host == b.Host && a.URL == b.URL &&
		a.Notes == b.Notes && slices.Equal(a.Tags, b.Tags) && slices.Equal(a.Fields, b.Fields) &&
		maps.Equal(a.Params, b.Params)
}

func mergeHistory(a, b []Revision, limit int) []Revision {
	if limit <= 0 {
		return nil
	}
	out := make([]Revision, 0, len(a)+len(b))
	for _, r := range append(slices.Clone(a), b...) {
		if !slices.ContainsFunc(out, func(got Revision) bool { return got.At.Equal(r.At) && sameValues(got, r) }) {
			out = append(out, r.Clone())
		}
	}
	slices.SortStableFunc(out, func(x, y Revision) int { return y.At.Compare(x.At) })
	return slices.Clip(out[:min(len(out), limit)])
}

func (t Type) Valid() bool {
	return slices.Contains(types, t)
}

func (e Entry) Validate() error {
	switch {
	case e.Path == "":
		return fmt.Errorf("%w: empty path", ErrInvalidEntry)
	case safetext.HasControl(e.Path) || safetext.HasControl(e.Name):
		return fmt.Errorf("%w: control character in path or name", ErrInvalidEntry)
	case !e.Type.Valid():
		return fmt.Errorf("%w: unknown type %q", ErrInvalidEntry, e.Type)
	}
	if err := ValidatePath(e.Path); err != nil {
		return err
	}
	for _, f := range e.Fields {
		if f.Name == "" {
			return fmt.Errorf("%w: empty field name", ErrInvalidEntry)
		}
		if safetext.HasControl(f.Name) {
			return fmt.Errorf("%w: control character in field name", ErrInvalidEntry)
		}
	}
	return nil
}

func ValidatePath(path string) error {
	if strings.HasPrefix(path, "-") {
		return fmt.Errorf("%w %q: starts with -", ErrBadPath, path)
	}
	if strings.HasPrefix(path, "/") {
		return fmt.Errorf("%w %q: starts with /", ErrBadPath, path)
	}
	for seg := range strings.SplitSeq(path, "/") {
		switch {
		case seg == "":
			return fmt.Errorf("%w %q: has an empty part", ErrBadPath, path)
		case seg == "." || seg == "..":
			return fmt.Errorf("%w %q: has a . or .. part", ErrBadPath, path)
		case seg != strings.TrimSpace(seg):
			return fmt.Errorf("%w %q: has spaces around a part", ErrBadPath, path)
		}
	}
	return nil
}

func comparePath(a, b Entry) int {
	return cmp.Compare(a.Path, b.Path)
}

func (e Entry) Clone() Entry {
	e.Tags = slices.Clone(e.Tags)
	e.Fields = slices.Clone(e.Fields)
	e.Params = maps.Clone(e.Params)
	e.History = slices.Clone(e.History)
	for i, r := range e.History {
		e.History[i] = r.Clone()
	}
	return e
}

const FieldEngine = "engine"

func (e Entry) Field(name string) (string, bool) {
	if i := slices.IndexFunc(e.Fields, func(f Field) bool { return f.Name == name }); i >= 0 {
		return e.Fields[i].Value, true
	}
	v, ok := e.Params[name]
	return v, ok
}

func (e Entry) WithRevision(r Revision) Entry {
	e.Name, e.Username, e.Host, e.URL, e.Notes = r.Name, r.Username, r.Host, r.URL, r.Notes
	e.Tags, e.Fields, e.Params = slices.Clone(r.Tags), slices.Clone(r.Fields), maps.Clone(r.Params)
	e.Updated, e.Machine, e.History = r.At, r.Machine, nil
	return e
}
