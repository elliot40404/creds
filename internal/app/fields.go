package app

import (
	"errors"
	"io"
	"slices"

	"github.com/elliot40404/creds/internal/envfile"
	"github.com/elliot40404/creds/internal/render"
	"github.com/elliot40404/creds/internal/vault"
)

const (
	FieldUsername = "username"
	FieldHost     = "host"
	FieldURL      = "url"
	FieldNotes    = "notes"
	FieldPassword = "password"
	FieldToken    = "token"
	FieldCommand  = "command"
	fieldKey      = "key"
	FieldPort     = "port"
	FieldDatabase = "database"
	FieldEngine   = vault.FieldEngine
	fieldScheme   = "scheme"
)

var ErrNoSecret = errors.New("entry has no secret field, use --field")

type FieldSpec struct {
	Key    string
	Secret bool
}

type ExtraSpec struct {
	Label  string
	Secret bool
	Check  func(string) error
}

var typeOrder = []vault.Type{
	vault.TypeLogin, vault.TypeAPI, vault.TypeDatabase, vault.TypeSSH,
	vault.TypeNote, vault.TypeCommand, vault.TypeEnv, vault.TypeGeneric,
}

var fieldSpecs = map[vault.Type][]FieldSpec{
	vault.TypeLogin:    {{Key: FieldUsername}, {Key: FieldPassword, Secret: true}, {Key: FieldURL}},
	vault.TypeAPI:      {{Key: FieldURL}, {Key: fieldKey, Secret: true}},
	vault.TypeDatabase: {{Key: FieldHost}, {Key: FieldUsername}},
	vault.TypeSSH:      {{Key: FieldHost}, {Key: FieldUsername}, {Key: FieldPort}, {Key: FieldPassword, Secret: true}},
	vault.TypeNote:     {{Key: FieldNotes}},
	vault.TypeCommand:  {{Key: FieldCommand}},
}

var engines = []string{render.Postgres, render.Redis, render.Mongo}

func Types() []vault.Type {
	return slices.Clone(typeOrder)
}

func TypeNames() []string {
	out := make([]string, len(typeOrder))
	for i, t := range typeOrder {
		out[i] = string(t)
	}
	return out
}

func Engines() []string {
	return slices.Clone(engines)
}

func FieldSpecs(t vault.Type) []FieldSpec {
	return slices.Clone(fieldSpecs[t])
}

func ExtraField(t vault.Type) ExtraSpec {
	if t == vault.TypeEnv {
		return ExtraSpec{Label: "Variable name", Secret: true, Check: checkEnvName}
	}
	return ExtraSpec{Label: "Extra field name"}
}

func checkEnvName(name string) error {
	return envfile.Write(io.Discard, []envfile.Var{{Key: name}})
}

func IsSpecField(t vault.Type, key string) bool {
	return slices.ContainsFunc(fieldSpecs[t], func(s FieldSpec) bool { return s.Key == key })
}

func BuiltinField(e *vault.Entry, key string) *string {
	switch key {
	case FieldUsername:
		return &e.Username
	case FieldHost:
		return &e.Host
	case FieldURL:
		return &e.URL
	case FieldNotes:
		return &e.Notes
	}
	return nil
}

func SetField(e *vault.Entry, key, val string, secret bool) {
	if p := BuiltinField(e, key); p != nil {
		*p = val
		return
	}
	i := slices.IndexFunc(e.Fields, func(f vault.Field) bool { return f.Name == key })
	switch {
	case i >= 0 && val == "":
		e.Fields = slices.Delete(e.Fields, i, i+1)
	case i >= 0:
		e.Fields[i].Value = val
		e.Fields[i].Secret = e.Fields[i].Secret || secret
	case val != "":
		e.Fields = append(e.Fields, vault.Field{Name: key, Value: val, Secret: secret})
	}
}

func SetConn(e *vault.Entry, engine, raw string) error {
	c, err := render.Parse(engine, raw)
	if err != nil {
		return err
	}
	e.Host, e.Username, e.Params = c.Host, c.Username, c.Params
	SetField(e, FieldEngine, engine, false)
	SetField(e, fieldScheme, c.Scheme, false)
	SetField(e, FieldPort, c.Port, false)
	SetField(e, FieldDatabase, c.Database, false)
	SetField(e, FieldPassword, c.Password, true)
	e.Fields = slices.DeleteFunc(e.Fields, func(f vault.Field) bool {
		return f.Name != FieldPassword && render.SecretParam(f.Name)
	})
	for k, v := range c.Params {
		if render.SecretParam(k) {
			SetField(e, k, v, true)
			delete(e.Params, k)
		}
	}
	return nil
}

func DefaultField(e vault.Entry) (string, error) {
	has := func(name string) bool {
		return slices.ContainsFunc(e.Fields, func(f vault.Field) bool { return f.Name == name })
	}
	for _, k := range []string{FieldPassword, FieldToken} {
		if has(k) {
			return k, nil
		}
	}
	if i := slices.IndexFunc(e.Fields, func(f vault.Field) bool { return f.Secret }); i >= 0 {
		return e.Fields[i].Name, nil
	}
	if e.Type == vault.TypeCommand && has(FieldCommand) {
		return FieldCommand, nil
	}
	return "", ErrNoSecret
}
