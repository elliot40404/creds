package cli

import (
	"errors"
	"fmt"
	"strings"

	"github.com/elliot40404/creds/internal/app"
	"github.com/elliot40404/creds/internal/envfile"
	"github.com/elliot40404/creds/internal/render"
	"github.com/elliot40404/creds/internal/vault"
)

var (
	ErrUnknownType    = errors.New("unknown entry type")
	ErrDuplicateField = errors.New("field already exists")

	errFieldName = errors.New("field name cannot contain =")
)

type form struct {
	p    app.Prompter
	edit bool
	skip func(string) bool
}

func (f form) skipped(key string) bool {
	return f.skip != nil && f.skip(key)
}

func (f form) ask(key string, secret bool, def string) (string, error) {
	if !secret {
		return f.p.Input(key, def)
	}
	label := key
	if f.edit && def != "" {
		label = key + " (blank keeps current)"
	}
	v, err := f.p.Password(label)
	if err != nil || v != "" {
		return v, err
	}
	return def, nil
}

func (f form) specs(e *vault.Entry) error {
	for _, s := range app.FieldSpecs(e.Type) {
		if f.skipped(s.Key) {
			continue
		}
		def, _ := app.Lookup(*e, s.Key)
		v, err := f.ask(s.Key, s.Secret, def)
		if err != nil {
			return err
		}
		app.SetField(e, s.Key, v, s.Secret)
	}
	return nil
}

func (f form) custom(e *vault.Entry) error {
	x := app.ExtraField(e.Type)
	for {
		name, err := app.Retry(f.p, func() (string, error) { return f.fieldName(e, x) }, errFieldName, ErrDuplicateField, envfile.ErrBadKey)
		if err != nil || name == "" {
			return err
		}
		secret, err := f.secret(x.Secret)
		if err != nil {
			return err
		}
		v, err := f.ask(name, secret, "")
		if err != nil {
			return err
		}
		app.SetField(e, name, v, secret)
	}
}

func (f form) secret(def bool) (bool, error) {
	if !def {
		return f.p.Confirm("Secret?")
	}
	return app.Retry(f.p, func() (bool, error) {
		ans, err := f.p.Input("Secret? [Y/n]", "")
		if err != nil {
			return false, err
		}
		switch strings.ToLower(ans) {
		case "", "y", "yes":
			return true, nil
		case "n", "no":
			return false, nil
		}
		return false, fmt.Errorf("%w: %q, type y or n", ErrBadChoice, ans)
	}, ErrBadChoice)
}

func (f form) fieldName(e *vault.Entry, x app.ExtraSpec) (string, error) {
	name, err := f.p.Input(x.Label+" (blank to finish)", "")
	switch {
	case err != nil || name == "":
		return "", err
	case strings.Contains(name, "="):
		return "", fmt.Errorf("%w: %s", errFieldName, name)
	}
	if _, err := app.Lookup(*e, name); err == nil || f.skipped(name) {
		return "", fmt.Errorf("%w: %s", ErrDuplicateField, name)
	}
	if x.Check != nil {
		return name, x.Check(name)
	}
	return name, nil
}

func (f form) database(e *vault.Entry) error {
	engine, raw, err := f.conn("")
	if err != nil {
		return err
	}
	return app.SetConn(e, engine, raw)
}

func (f form) conn(engine string) (string, string, error) {
	raw, err := f.p.Password("Connection string")
	if err != nil || engine != "" {
		return engine, raw, err
	}
	if found, ok := render.EngineFor(raw); ok {
		return found, raw, f.p.Show("engine", found+" (from connection string)")
	}
	engines := app.Engines()
	i, err := f.p.Select("Engine", engines)
	if err != nil {
		return "", "", err
	}
	return engines[i], raw, nil
}

func pickType(p app.Prompter, flag string) (vault.Type, error) {
	if flag != "" {
		t := vault.Type(flag)
		if !t.Valid() {
			return "", fmt.Errorf("%w: %q", ErrUnknownType, flag)
		}
		return t, nil
	}
	names := app.TypeNames()
	i, err := p.Select("Type", names)
	if err != nil {
		return "", err
	}
	return vault.Type(names[i]), nil
}

func askPath(p app.Prompter, prompt, def string) (string, error) {
	return app.Retry(p, func() (string, error) {
		v, err := p.Input(prompt, def)
		if err != nil || v == "" {
			return v, err
		}
		return v, vault.ValidatePath(v)
	}, vault.ErrBadPath)
}
