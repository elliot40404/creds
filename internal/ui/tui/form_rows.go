package tui

import (
	"maps"
	"slices"

	"github.com/elliot40404/creds/internal/render"
	"github.com/elliot40404/creds/internal/vault"
)

func paramRows(params map[string]string) []formRow {
	rows := make([]formRow, 0, len(params))
	for _, k := range slices.Sorted(maps.Keys(params)) {
		rows = append(rows, formRow{kind: kindParam, key: k, input: newInput(params[k], render.SecretParam(k))})
	}
	return rows
}

func (r formRow) removable() bool {
	return r.custom || r.kind == kindParam
}

func (f formScreen) removeRow() formScreen {
	r := f.rows[f.focus]
	if !r.removable() {
		f.errMsg = r.key + " cannot be removed. only custom fields and params can"
		return f
	}
	if r.kind == kindField {
		f.removed = append(slices.Clip(f.removed), r.key)
	}
	f.rows = slices.Delete(slices.Clone(f.rows), f.focus, f.focus+1)
	f.focus, f.errMsg = min(f.focus, len(f.rows)-1), ""
	return f
}

func dropFields(e *vault.Entry, names []string) {
	e.Fields = slices.DeleteFunc(e.Fields, func(fd vault.Field) bool { return slices.Contains(names, fd.Name) })
}

func setParam(e *vault.Entry, key, value string) {
	if value == "" {
		return
	}
	if e.Params == nil {
		e.Params = map[string]string{}
	}
	e.Params[key] = value
}
