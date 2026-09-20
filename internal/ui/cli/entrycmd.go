package cli

import (
	"io"
	"strings"

	"github.com/elliot40404/creds/internal/app"
	"github.com/elliot40404/creds/internal/vault"
	"github.com/spf13/cobra"
)

func entryCommands(env Env) []*cobra.Command {
	return []*cobra.Command{
		addCmd(env), getCmd(env), copyCmd(env), listCmd(env), searchCmd(env), editCmd(env), rmCmd(env),
	}
}

func addCmd(env Env) *cobra.Command {
	var typ string
	var flags *entryFlags
	c := &cobra.Command{
		Use:               "add <path>",
		Short:             "Add an entry",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: env.completeNewPath,
		RunE: env.run(func(s *app.Service, args []string) error {
			if err := s.Unlock(); err != nil {
				return err
			}
			e, err := newEntry(env.Prompter, env.In, args[0], typ, flags)
			if err != nil {
				return err
			}
			if _, err := s.Add(e); err != nil {
				return err
			}
			env.note("added %s", e.Path)
			return nil
		}),
	}
	c.Flags().StringVar(&typ, "type", "", "entry type: "+strings.Join(app.TypeNames(), "|"))
	flags = bindEntryFlags(c)
	return c
}

func newEntry(p app.Prompter, in io.Reader, path, typ string, flags *entryFlags) (vault.Entry, error) {
	if flags.used() {
		return flagEntry(p, in, path, typ, flags)
	}
	t, err := pickType(p, typ)
	if err != nil {
		return vault.Entry{}, err
	}
	e := vault.Entry{Path: path, Type: t}
	f := form{p: p}
	if t == vault.TypeDatabase {
		err = f.database(&e)
	} else {
		err = f.specs(&e)
	}
	if err != nil {
		return vault.Entry{}, err
	}
	return e, f.custom(&e)
}

func editCmd(env Env) *cobra.Command {
	var flags *entryFlags
	var rename string
	c := &cobra.Command{
		Use:               "edit <path>",
		Short:             "Edit an entry",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: env.completePaths,
		RunE: env.run(func(s *app.Service, args []string) error {
			e, err := s.Get(args[0])
			if err != nil {
				return err
			}
			edit := func() error { return editEntry(form{p: env.Prompter, edit: true}, &e) }
			if flags.used() || rename != "" {
				edit = func() error {
					if rename != "" {
						e.Path = rename
					}
					return flags.apply(&e, env.In, env.Prompter)
				}
			}
			if err := edit(); err != nil {
				return err
			}
			if _, err := s.Update(args[0], e); err != nil {
				return err
			}
			env.note("updated %s", e.Path)
			return nil
		}),
	}
	flags = bindEntryFlags(c)
	c.Flags().StringVar(&rename, flagRename, "", "move the entry to this path")
	return c
}

func editEntry(f form, e *vault.Entry) error {
	if !f.skipped(flagRename) {
		path, err := askPath(f.p, "path", e.Path)
		if err != nil {
			return err
		}
		e.Path = path
	}
	if err := f.specs(e); err != nil {
		return err
	}
	for i := range e.Fields {
		fd := &e.Fields[i]
		if app.IsSpecField(e.Type, fd.Name) || f.skipped(fd.Name) {
			continue
		}
		v, err := f.ask(fd.Name, fd.Secret, fd.Value)
		if err != nil {
			return err
		}
		fd.Value = v
	}
	return f.custom(e)
}

func rmCmd(env Env) *cobra.Command {
	var yes bool
	c := &cobra.Command{
		Use:               "rm <path>",
		Short:             "Remove an entry",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: env.completePaths,
		RunE: env.run(func(s *app.Service, args []string) error {
			if _, err := s.Get(args[0]); err != nil {
				return err
			}
			if err := env.confirm(yes, "Remove "+args[0]+"?"); err != nil {
				return err
			}
			if err := s.Delete(args[0]); err != nil {
				return err
			}
			env.note("removed %s", args[0])
			return nil
		}),
	}
	yesFlag(c, &yes)
	return c
}

func flagEntry(p app.Prompter, in io.Reader, path, typ string, flags *entryFlags) (vault.Entry, error) {
	if typ == "" {
		return vault.Entry{}, errNeedType
	}
	t, err := pickType(nil, typ)
	if err != nil {
		return vault.Entry{}, err
	}
	e := vault.Entry{Path: path, Type: t}
	return e, flags.apply(&e, in, p)
}
