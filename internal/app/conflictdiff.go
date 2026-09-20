package app

import (
	"context"
	"maps"
	"slices"
	"strings"

	"github.com/elliot40404/creds/internal/render"
	"github.com/elliot40404/creds/internal/vault"
)

const deletedValue = "(deleted)"

type FieldDiff struct {
	Name   string
	Mine   string
	Theirs string
	Secret bool
}

type diffValue struct {
	value  string
	secret bool
}

func (s *Service) ConflictDiff(path string) ([]FieldDiff, error) {
	sy, err := s.syncer()
	if err != nil {
		return nil, err
	}
	ours, theirs, err := sy.ConflictSides(context.Background(), path)
	if err != nil {
		return nil, err
	}
	return diffEntries(ours, theirs), nil
}

func diffEntries(ours, theirs *vault.Entry) []FieldDiff {
	if ours == nil || theirs == nil {
		return []FieldDiff{{Name: "entry", Mine: presence(ours), Theirs: presence(theirs)}}
	}
	a, b := entryValues(*ours), entryValues(*theirs)
	var out []FieldDiff
	keys := maps.Clone(a)
	maps.Copy(keys, b)
	for _, k := range slices.Sorted(maps.Keys(keys)) {
		x, y := a[k], b[k]
		if x == y {
			continue
		}
		d := FieldDiff{Name: k, Secret: x.secret || y.secret}
		if !d.Secret {
			d.Mine, d.Theirs = x.value, y.value
		}
		out = append(out, d)
	}
	return out
}

func presence(e *vault.Entry) string {
	if e == nil {
		return deletedValue
	}
	return "kept"
}

func entryValues(e vault.Entry) map[string]diffValue {
	out := map[string]diffValue{}
	add := func(k, v string, secret bool) {
		if v != "" {
			out[k] = diffValue{v, secret}
		}
	}
	add("path", e.Path, false)
	add("type", string(e.Type), false)
	add("name", e.Name, false)
	add(FieldUsername, e.Username, false)
	add(FieldHost, e.Host, false)
	add(FieldURL, e.URL, false)
	add(FieldNotes, e.Notes, false)
	add("tags", strings.Join(e.Tags, ","), false)
	for _, f := range e.Fields {
		add(f.Name, f.Value, f.Secret)
	}
	for k, v := range e.Params {
		add(k, v, render.SecretParam(k))
	}
	return out
}
