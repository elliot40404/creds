package cli

import (
	"errors"
	"slices"
	"strings"

	"github.com/elliot40404/creds/internal/app"
	"github.com/elliot40404/creds/internal/vault"
)

const flagRename = "rename"

var errNeedPath = errors.New("entry path is required")

var builtinFlags = []string{app.FieldUsername, app.FieldHost, app.FieldURL, app.FieldNotes}

func askAdd(iv *interview) error {
	if err := iv.arg(0, iv.path("Entry path"+iv.folderHint())); err != nil {
		return err
	}
	if len(iv.args) == 0 {
		return errNeedPath
	}
	t, err := pickType(iv.p, iv.str("type"))
	if err != nil {
		return err
	}
	if err := iv.set("type", string(t)); err != nil {
		return err
	}
	e := vault.Entry{Path: iv.args[0], Type: t}
	f := iv.form(false)
	if err := iv.typedSecrets(f); err != nil {
		return err
	}
	if err := iv.addFields(f, &e); err != nil {
		return err
	}
	if err := f.custom(&e); err != nil {
		return err
	}
	return iv.saveForm(e, vault.Entry{Path: e.Path, Type: t})
}

func (iv *interview) addFields(f form, e *vault.Entry) error {
	switch {
	case e.Type != vault.TypeDatabase:
		return f.specs(e)
	case iv.changed("conn"):
		return nil
	}
	return iv.askConn(f)
}

func askEdit(iv *interview) error {
	if err := iv.entryArg(); err != nil {
		return err
	}
	e, err := iv.entry()
	if err != nil {
		return err
	}
	f := iv.form(true)
	if err := iv.typedSecrets(f); err != nil {
		return err
	}
	before := e
	before.Fields = slices.Clone(e.Fields)
	if err := editEntry(f, &e); err != nil {
		return err
	}
	if e.Path != before.Path {
		if err := iv.set(flagRename, e.Path); err != nil {
			return err
		}
	}
	return iv.saveForm(e, before)
}

func (iv *interview) form(edit bool) form {
	return form{p: iv.p, edit: edit, skip: iv.typedField}
}

func (iv *interview) typedField(key string) bool {
	if key == flagRename || slices.Contains(builtinFlags, key) {
		return iv.changed(key)
	}
	has := func(kv string) bool {
		k, _, _ := strings.Cut(kv, "=")
		return k == key
	}
	return slices.ContainsFunc(iv.list("field"), has) || slices.ContainsFunc(iv.list("secret-field"), has)
}

func (iv *interview) typedSecrets(f form) error {
	if iv.changed("conn") {
		_, raw, err := f.conn(iv.str(app.FieldEngine))
		if err != nil {
			return err
		}
		iv.conn = []string{raw}
	}
	for _, kv := range iv.list("secret-field") {
		k, _, _ := strings.Cut(kv, "=")
		v, err := iv.p.Password(k)
		if err != nil {
			return err
		}
		iv.stdin = append(iv.stdin, v)
	}
	return nil
}

func (iv *interview) askConn(f form) error {
	engine, raw, err := f.conn(iv.str(app.FieldEngine))
	if err != nil {
		return err
	}
	iv.conn = []string{raw}
	return firstErr(iv.set(app.FieldEngine, engine), func() error { return iv.set("conn", stdinValue) })
}

func (iv *interview) saveForm(e, before vault.Entry) error {
	for _, k := range builtinFlags {
		if v := *app.BuiltinField(&e, k); v != *app.BuiltinField(&before, k) {
			if err := iv.set(k, v); err != nil {
				return err
			}
		}
	}
	for _, fd := range e.Fields {
		if err := iv.fieldFlag(fd, before); err != nil {
			return err
		}
	}
	return nil
}

func (iv *interview) fieldFlag(fd vault.Field, before vault.Entry) error {
	if i := slices.IndexFunc(before.Fields, func(b vault.Field) bool { return b.Name == fd.Name }); i >= 0 && before.Fields[i] == fd {
		return nil
	}
	if !fd.Secret {
		return iv.set("field", fd.Name+"="+fd.Value)
	}
	iv.stdin = append(iv.stdin, fd.Value)
	return iv.set("secret-field", fd.Name)
}
