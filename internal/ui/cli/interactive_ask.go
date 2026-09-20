package cli

import (
	"errors"
	"slices"

	"github.com/elliot40404/creds/internal/app"
	"github.com/elliot40404/creds/internal/search"
	"github.com/elliot40404/creds/internal/vault"
)

var errNoMatch = errors.New("no entries match")

func (iv *interview) service() (*app.Service, error) {
	if iv.svc != nil {
		return iv.svc, nil
	}
	s, err := iv.env.service(iv.p)
	if err != nil {
		return nil, err
	}
	s.Spawn = nil
	iv.svc = s
	return s, nil
}

func (iv *interview) unlock() error {
	s, err := iv.service()
	if err != nil {
		return err
	}
	return s.Unlock()
}

func (iv *interview) changed(name string) bool {
	f := iv.cmd.Flag(name)
	return f != nil && f.Changed
}

func (iv *interview) anyChanged(names ...string) bool {
	return slices.ContainsFunc(names, iv.changed)
}

func (iv *interview) set(name, value string) error {
	return iv.cmd.Flags().Set(name, value)
}

func (iv *interview) str(name string) string {
	if !iv.changed(name) {
		return ""
	}
	return iv.cmd.Flag(name).Value.String()
}

func (iv *interview) list(name string) []string {
	v, _ := iv.cmd.Flags().GetStringArray(name)
	return v
}

func (iv *interview) arg(i int, ask func() (string, error)) error {
	if len(iv.args) > i {
		return nil
	}
	v, err := ask()
	if err != nil || v == "" {
		return err
	}
	iv.args = append(iv.args, v)
	return nil
}

func (iv *interview) input(prompt, def string) func() (string, error) {
	return func() (string, error) { return iv.p.Input(prompt, def) }
}

func (iv *interview) path(prompt string) func() (string, error) {
	return func() (string, error) { return askPath(iv.p, prompt, "") }
}

func (iv *interview) text(name, prompt, def string) error {
	if iv.changed(name) {
		return nil
	}
	v, err := iv.p.Input(prompt, def)
	if err != nil || v == "" {
		return err
	}
	return iv.set(name, v)
}

func (iv *interview) confirm(name, prompt string) error {
	if iv.changed(name) {
		return nil
	}
	ok, err := iv.p.Confirm(prompt)
	if err != nil || !ok {
		return err
	}
	return iv.set(name, "true")
}

func (iv *interview) choose(prompt string, options []string) (string, error) {
	i, err := iv.p.Select(prompt, options)
	if err != nil {
		return "", err
	}
	return options[i], nil
}

func (iv *interview) entryArg() error {
	return iv.arg(0, func() (string, error) { return iv.pickEntry("") })
}

func (iv *interview) pickEntry(extra string) (string, error) {
	items, err := iv.filtered("")
	if err != nil {
		return "", err
	}
	return iv.pickFrom(items, extra)
}

func (iv *interview) pickFrom(items []search.Summary, extra string) (string, error) {
	var opts []string
	if extra != "" {
		opts = append(opts, extra)
	}
	for _, it := range items {
		opts = append(opts, it.Path)
	}
	if len(opts) == 0 {
		return "", errNoMatch
	}
	v, err := iv.choose("Entry", opts)
	if err != nil || v == extra {
		return "", err
	}
	return v, nil
}

func (iv *interview) filtered(query string) ([]search.Summary, error) {
	s, err := iv.service()
	if err != nil {
		return nil, err
	}
	if query == "" {
		if query, err = iv.p.Input("Search (blank for all, type:<type> engine:<engine> tag:<tag> to filter)", ""); err != nil {
			return nil, err
		}
	}
	return s.Search(query, search.SortPath)
}

func fieldNames(e vault.Entry) []string {
	var out []string
	for _, k := range []string{app.FieldUsername, app.FieldHost, app.FieldURL, app.FieldNotes} {
		if *app.BuiltinField(&e, k) != "" {
			out = append(out, k)
		}
	}
	for _, f := range e.Fields {
		out = append(out, f.Name)
	}
	return out
}

const maxFolderHint = 60

func (iv *interview) folderHint() string {
	if iv.svc == nil {
		return ""
	}
	items, err := iv.svc.List()
	if err != nil {
		return ""
	}
	folders := search.Prefixes(search.Paths(items))
	if len(folders) == 0 {
		return ""
	}
	out := ""
	for i, f := range folders {
		next := f
		if i > 0 {
			next = out + ", " + f
		}
		if len(next) > maxFolderHint {
			return " (folders: " + out + ", …)"
		}
		out = next
	}
	return " (folders: " + out + ")"
}
