package cli

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"

	"github.com/elliot40404/creds/internal/app"
	"github.com/elliot40404/creds/internal/render"
	"github.com/elliot40404/creds/internal/vault"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

const stdinValue = "-"

var (
	errSecretInArgs  = errors.New("secret values are not accepted as arguments")
	errFieldFormat   = errors.New("field must be key=value")
	errNeedType      = errors.New("type is required when adding by flags")
	errNeedEngine    = errors.New("cannot tell the database engine from the connection string")
	errEngineAlone   = errors.New("--engine needs --conn")
	errNotDatabase   = errors.New("--conn and --engine only apply to database entries")
	errShortStdin    = errors.New("stdin has fewer lines than secret values asked for")
	errSecretBuiltin = errors.New("builtin fields cannot be secret")
)

var entryFlagNames = []string{"name", "username", "host", "url", "notes", "tag", "field", "secret-field", "engine", "conn"}

type entryFlags struct {
	cmd                                      *cobra.Command
	name, username, host, url, notes, engine string
	conn                                     string
	tags, fields, secrets                    []string
}

func bindEntryFlags(c *cobra.Command) *entryFlags {
	f := &entryFlags{cmd: c}
	fl := c.Flags()
	fl.StringVar(&f.name, "name", "", "display name")
	fl.StringVar(&f.username, app.FieldUsername, "", "username")
	fl.StringVar(&f.host, app.FieldHost, "", "host")
	fl.StringVar(&f.url, app.FieldURL, "", "url")
	fl.StringVar(&f.notes, app.FieldNotes, "", "notes")
	fl.StringArrayVar(&f.tags, "tag", nil, "tag, repeat for more (replaces all tags)")
	fl.StringArrayVar(&f.fields, "field", nil, "visible field as key=value, repeat for more")
	fl.StringArrayVar(&f.secrets, "secret-field", nil, "secret field name, value read from stdin one line each in order")
	fl.StringVar(&f.engine, app.FieldEngine, "", "database engine: "+strings.Join(app.Engines(), "|"))
	fl.StringVar(&f.conn, "conn", "", "pass - to read the database connection string from stdin (read before secret fields)")
	return f
}

func (f *entryFlags) used() bool {
	return slices.ContainsFunc(entryFlagNames, f.changed)
}

func (f *entryFlags) changed(name string) bool {
	return f.cmd.Flags().Changed(name)
}

func (f *entryFlags) apply(e *vault.Entry, in io.Reader, p app.Prompter) error {
	if err := f.check(e); err != nil {
		return err
	}
	read := secretSource(in, p, isTerminal(in))
	if err := f.applyConn(e, read); err != nil {
		return err
	}
	for _, p := range []struct {
		flag string
		dst  *string
		val  string
	}{
		{"name", &e.Name, f.name},
		{app.FieldUsername, &e.Username, f.username},
		{app.FieldHost, &e.Host, f.host},
		{app.FieldURL, &e.URL, f.url},
		{app.FieldNotes, &e.Notes, f.notes},
	} {
		if f.changed(p.flag) {
			*p.dst = p.val
		}
	}
	if f.changed("tag") {
		e.Tags = slices.Clone(f.tags)
	}
	for _, kv := range f.fields {
		k, v, _ := strings.Cut(kv, "=")
		app.SetField(e, k, v, false)
	}
	for _, k := range f.secrets {
		name := strings.TrimSuffix(k, "="+stdinValue)
		v, err := read(name)
		if err != nil {
			return err
		}
		app.SetField(e, name, v, true)
	}
	return nil
}

func (f *entryFlags) check(e *vault.Entry) error {
	if f.changed("conn") && f.conn != stdinValue {
		return fmt.Errorf("%w: --conn", errSecretInArgs)
	}
	if (f.changed("conn") || f.changed(app.FieldEngine)) && e.Type != vault.TypeDatabase {
		return errNotDatabase
	}
	if f.changed(app.FieldEngine) && !f.changed("conn") {
		return errEngineAlone
	}
	for _, k := range f.secrets {
		name, v, ok := strings.Cut(k, "=")
		if app.BuiltinField(e, name) != nil {
			return fmt.Errorf("%w: --secret-field %s", errSecretBuiltin, name)
		}
		if name == "" || ok && v != stdinValue {
			return fmt.Errorf("%w: --secret-field %s", errSecretInArgs, name)
		}
	}
	for _, kv := range f.fields {
		k, _, ok := strings.Cut(kv, "=")
		if !ok || k == "" {
			return fmt.Errorf("%w: %s", errFieldFormat, k)
		}
		if isSecretField(*e, k) {
			return fmt.Errorf("%w: %s is a secret field", errSecretInArgs, k)
		}
	}
	return nil
}

func isSecretField(e vault.Entry, name string) bool {
	secret := func(s app.FieldSpec) bool { return s.Key == name && s.Secret }
	has := func(fd vault.Field) bool { return fd.Name == name && fd.Secret }
	return name == app.FieldPassword || slices.ContainsFunc(app.FieldSpecs(e.Type), secret) || slices.ContainsFunc(e.Fields, has)
}

func (f *entryFlags) applyConn(e *vault.Entry, read func(string) (string, error)) error {
	if !f.changed("conn") {
		return nil
	}
	raw, err := read("connection string")
	if err != nil {
		return err
	}
	engine := f.engine
	if engine == "" {
		engine, _ = render.EngineFor(raw)
	}
	if engine == "" {
		engine, _ = app.Lookup(*e, app.FieldEngine)
	}
	if engine == "" {
		return errNeedEngine
	}
	return app.SetConn(e, engine, raw)
}

func stdinLines(in io.Reader) func() (string, error) {
	if in == nil {
		in = strings.NewReader("")
	}
	sc := bufio.NewScanner(in)
	return func() (string, error) {
		if sc.Scan() {
			return strings.TrimSuffix(sc.Text(), "\r"), nil
		}
		if err := sc.Err(); err != nil {
			return "", err
		}
		return "", errShortStdin
	}
}

func secretSource(in io.Reader, p app.Prompter, tty bool) func(string) (string, error) {
	if tty {
		return p.Password
	}
	lines := stdinLines(in)
	return func(string) (string, error) { return lines() }
}

func isTerminal(v any) bool {
	f, ok := v.(*os.File)
	return ok && term.IsTerminal(int(f.Fd()))
}
