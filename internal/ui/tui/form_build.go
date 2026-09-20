package tui

import (
	"slices"

	tea "charm.land/bubbletea/v2"
	"github.com/elliot40404/creds/internal/app"
	"github.com/elliot40404/creds/internal/vault"
)

func (f formScreen) submit() (Screen, tea.Cmd) {
	f.rows = slices.Clone(f.rows)
	e, bad := f.build()
	if bad >= 0 {
		f.focus = bad
		f.errMsg = ""
		return f, nil
	}
	f.saving, f.errMsg = true, ""
	return f, send(FormResult{Entry: e})
}

func (f *formScreen) build() (vault.Entry, int) {
	e := vault.Entry{Type: f.typ}
	if f.edit {
		e = f.orig.Clone()
		dropFields(&e, f.removed)
		e.Params = nil
	}
	for i := range f.rows {
		r := &f.rows[i]
		r.err = ""
		switch r.kind {
		case kindPath:
			e.Path = r.input.String()
			if e.Path == "" {
				r.err = "path is empty. type a path like web/github"
			}
		case kindEngine:
		case kindParam:
			setParam(&e, r.key, r.input.String())
		case kindConn:
			r.err = f.setConn(&e, r.input.String())
		default:
			r.err = f.setRow(&e, *r)
		}
		if r.err != "" {
			return vault.Entry{}, i
		}
	}
	return e, -1
}

func (f formScreen) setConn(e *vault.Entry, raw string) string {
	if raw == "" {
		return "connection string is empty. paste one like postgres://user:pass@host:5432/db"
	}
	if err := app.SetConn(e, f.engines[f.engine], raw); err != nil {
		return err.Error() + ". check the engine and paste a full connection string"
	}
	return ""
}

func (f formScreen) setRow(e *vault.Entry, r formRow) string {
	v, secret := r.input.String(), r.input.secret
	if !f.edit && r.custom {
		if _, err := app.Lookup(*e, r.key); err == nil {
			return "field " + r.key + " already exists. use another name"
		}
	}
	if r.keep && v == "" {
		markSecret(e, r.key, secret)
		return ""
	}
	app.SetField(e, r.key, v, secret)
	markSecret(e, r.key, secret)
	return ""
}

func markSecret(e *vault.Entry, key string, secret bool) {
	if i := slices.IndexFunc(e.Fields, func(fd vault.Field) bool { return fd.Name == key }); i >= 0 {
		e.Fields[i].Secret = secret
	}
}
