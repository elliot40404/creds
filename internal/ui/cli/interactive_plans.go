package cli

import (
	"slices"
	"strings"

	"github.com/elliot40404/creds/internal/app"
	"github.com/elliot40404/creds/internal/clipboard"
	"github.com/elliot40404/creds/internal/vault"
)

const (
	actionEntry   = "whole entry"
	actionField   = "one field"
	actionAs      = "database format"
	actionFormats = "list database formats"
	projectEntry  = "(use .creds.toml in this folder)"
)

var clipKey = clipboard.ClearCommand

var entryActions = []string{"get", "copy", "edit", "rm"}

type plan struct {
	asks  []string
	skip  []string
	run   func(*interview) error
	vault bool
}

func plans() map[string]plan {
	yes := []string{"yes"}
	entryForm := []string{"name", "tag", "field", "secret-field", "engine", "conn"}
	layout := []string{flagInline, flagFullscreen, flagHeight, flagAltScreen}
	return map[string]plan{
		"":              {skip: layout, run: (*interview).pickCommand},
		"remote":        {run: (*interview).pickCommand},
		"init":          {},
		"unlock":        {},
		"lock":          {},
		"recover":       {},
		"passwd":        {},
		"pick":          {skip: layout},
		"setup":         {},
		"agents":        {},
		"paths":         {},
		"version":       {},
		"remote remove": {},
		"sync":          {skip: []string{"quiet"}},
		clipKey:         {skip: []string{"after", "mode"}},
		"add":           {asks: append([]string{"<path>", "type"}, builtinFlags...), skip: entryForm, run: askAdd, vault: true},
		"edit":          {asks: append([]string{"<path>", flagRename}, builtinFlags...), skip: entryForm, run: askEdit, vault: true},
		"get":           {asks: []string{"<path>", "show", "field", "as", "formats"}, skip: []string{"shell"}, run: askGet, vault: true},
		"copy":          {asks: []string{"<path>", "field", "as"}, skip: []string{"osc52", "native", "shell"}, run: askCopy, vault: true},
		"rm":            {asks: []string{"<path>"}, skip: yes, run: askEntryOnly, vault: true},
		"list":          {skip: []string{"sort"}, run: askList, vault: true},
		"search":        {asks: []string{"<query>"}, skip: []string{"sort"}, run: askList, vault: true},
		"env":           {asks: []string{"[path]", "output"}, skip: []string{"yes", "format"}, run: askEnv, vault: true},
		"run":           {asks: []string{"[path]", "<command>", "[args...]", "lock"}, run: askRun, vault: true},
		"import-env":    {asks: []string{"<file>", "<path>"}, run: askImportEnv, vault: true},
		"trust":         {asks: []string{"[dir]"}, skip: yes, run: askTrust},
		"trust list":    {},
		"trust remove":  {asks: []string{"<file>"}, skip: yes, run: askTrustFile},
		"export":        {asks: []string{"plain", "encrypted", "format", "output", "<file>"}, skip: []string{"confirm-plaintext", "yes"}, run: askExport},
		"import":        {asks: []string{"<file.json>", "overwrite"}, skip: yes, run: askImport, vault: true},
		"join":          {asks: []string{"<url>"}, skip: yes, run: askURL},
		"remote add":    {asks: []string{"<url>"}, run: askURL},
		"status":        {run: askStatus},
		"config":        {run: askConfig},
		"config edit":   {},
		"config get":    {asks: []string{"<key>"}, run: askConfigKey},
		"config unset":  {asks: []string{"<key>"}, run: askConfigKey},
		"config set":    {asks: []string{"<key>", "<value>"}, run: askConfigSet},
		"resolve":       {asks: []string{"<path>", "mine", "theirs"}, skip: yes, run: askResolve, vault: true},
	}
}

func askEntryOnly(iv *interview) error {
	return iv.entryArg()
}

func askList(iv *interview) error {
	items, err := iv.filtered(strings.Join(iv.args, " "))
	if err != nil {
		return err
	}
	path, err := iv.pickFrom(items, "")
	if err != nil {
		return err
	}
	action, err := iv.choose("Action", entryActions)
	if err != nil {
		return err
	}
	return iv.switchTo([]string{action}, path)
}

func askGet(iv *interview) error {
	if err := iv.entryArg(); err != nil || iv.anyChanged("field", "as", "formats") {
		return err
	}
	e, err := iv.entry()
	if err != nil {
		return err
	}
	opts := []string{actionEntry, actionField}
	if e.Type == vault.TypeDatabase {
		opts = append(opts, actionAs, actionFormats)
	}
	action, err := iv.choose("Print", opts)
	if err != nil {
		return err
	}
	switch action {
	case actionEntry:
		return iv.confirm("show", "Reveal secret values?")
	case actionField:
		return iv.pickField(e)
	case actionAs:
		return iv.pickFormat()
	}
	return iv.set("formats", "true")
}

func askCopy(iv *interview) error {
	if err := iv.entryArg(); err != nil || iv.anyChanged("field", "as") {
		return err
	}
	e, err := iv.entry()
	if err != nil {
		return err
	}
	if e.Type == vault.TypeDatabase {
		action, err := iv.choose("Copy", []string{actionField, actionAs})
		if err != nil || action == actionAs {
			return firstErr(err, iv.pickFormat)
		}
	}
	return iv.pickField(e)
}

func firstErr(err error, next func() error) error {
	if err != nil {
		return err
	}
	return next()
}

func (iv *interview) entry() (vault.Entry, error) {
	s, err := iv.service()
	if err != nil {
		return vault.Entry{}, err
	}
	return s.Get(iv.args[0])
}

func (iv *interview) pickField(e vault.Entry) error {
	names := fieldNames(e)
	if def, err := app.DefaultField(e); err == nil {
		names = append([]string{def}, slices.DeleteFunc(names, func(n string) bool { return n == def })...)
	}
	if len(names) == 0 {
		return app.ErrNoField
	}
	v, err := iv.choose("Field", names)
	if err != nil {
		return err
	}
	return iv.set("field", v)
}

func (iv *interview) pickFormat() error {
	s, err := iv.service()
	if err != nil {
		return err
	}
	formats, err := s.Formats(iv.args[0])
	if err != nil {
		return err
	}
	v, err := iv.choose("Format", formats)
	if err != nil {
		return err
	}
	return iv.set("as", v)
}

func askEnv(iv *interview) error {
	if err := iv.optionalEntry(); err != nil {
		return err
	}
	return iv.text("output", "Write to file (blank prints)", "")
}

func (iv *interview) optionalEntry() error {
	return iv.arg(0, func() (string, error) { return iv.pickEntry(projectEntry) })
}

func askRun(iv *interview) error {
	if !iv.dash {
		if err := iv.optionalEntry(); err != nil {
			return err
		}
		line, err := iv.p.Input("Command to run", "")
		if err != nil {
			return err
		}
		iv.dash, iv.tail = true, strings.Fields(line)
	}
	return iv.confirm("lock", "End the session before the command starts?")
}

func askImportEnv(iv *interview) error {
	if err := iv.arg(0, iv.input(".env file", ".env")); err != nil {
		return err
	}
	return iv.arg(1, iv.path("New entry path"))
}

func askTrust(iv *interview) error {
	return iv.arg(0, iv.input("Project folder", "."))
}

func askTrustFile(iv *interview) error {
	return iv.arg(0, func() (string, error) {
		s, err := iv.service()
		if err != nil {
			return "", err
		}
		files, err := s.TrustList()
		if err != nil {
			return "", err
		}
		if len(files) == 0 {
			return "", app.ErrUntrustedProject
		}
		return iv.choose("Project file", files)
	})
}

func askExport(iv *interview) error {
	if !iv.anyChanged("plain", "encrypted") {
		kind, err := iv.choose("Export", []string{"encrypted", "plain"})
		if err != nil {
			return err
		}
		if err := iv.set(kind, "true"); err != nil {
			return err
		}
	}
	if iv.changed("plain") {
		if err := iv.unlock(); err != nil {
			return err
		}
	}
	if iv.changed("plain") && !iv.changed("format") {
		v, err := iv.choose("Format", []string{app.FormatJSON, app.FormatCSV})
		if err != nil {
			return err
		}
		if err := iv.set("format", v); err != nil {
			return err
		}
	}
	return iv.text("output", "Output file", "")
}

func askImport(iv *interview) error {
	if err := iv.arg(0, iv.input("Export file", "")); err != nil {
		return err
	}
	if iv.changed("overwrite") {
		return nil
	}
	ok, err := iv.p.Confirm("Replace entries that have the same path?")
	if err != nil || !ok {
		return err
	}
	return firstErr(iv.set("overwrite", "true"), func() error { return iv.set("yes", "true") })
}

func askURL(iv *interview) error {
	return iv.arg(0, iv.input("Git remote URL", ""))
}
