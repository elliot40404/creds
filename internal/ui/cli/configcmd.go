package cli

import (
	"fmt"

	"github.com/elliot40404/creds/internal/app"
	"github.com/elliot40404/creds/internal/safetext"
	"github.com/spf13/cobra"
)

func configCommands(env Env) []*cobra.Command {
	c := &cobra.Command{
		Use:   "config",
		Short: "Show and change settings",
		Args:  cobra.NoArgs,
		RunE:  env.run(func(s *app.Service, _ []string) error { return env.printConfig(s) }),
	}
	c.AddCommand(configGetCmd(env), configSetCmd(env), configUnsetCmd(env), configEditCmd(env))
	return []*cobra.Command{c}
}

func configGetCmd(env Env) *cobra.Command {
	return &cobra.Command{
		Use:   "get <key>",
		Short: "Print one setting",
		Args:  cobra.ExactArgs(1),
		RunE: env.run(func(s *app.Service, args []string) error {
			v, err := s.ConfigGet(args[0])
			if err != nil {
				return err
			}
			if env.jsonMode() {
				return env.writeJSON(app.ConfigField{Key: args[0], Value: v})
			}
			_, err = fmt.Fprintln(env.Out, safetext.Line(v))
			return err
		}),
	}
}

func configSetCmd(env Env) *cobra.Command {
	return &cobra.Command{
		Use:   "set <key> <value>",
		Short: "Change one setting and save it",
		Args:  cobra.ExactArgs(2),
		RunE: env.run(func(s *app.Service, args []string) error {
			if err := s.ConfigSet(args[0], args[1]); err != nil {
				return err
			}
			env.note("%s set to %s", args[0], args[1])
			return nil
		}),
	}
}

func configUnsetCmd(env Env) *cobra.Command {
	return &cobra.Command{
		Use:   "unset <key>",
		Short: "Remove a render format override",
		Args:  cobra.ExactArgs(1),
		RunE: env.run(func(s *app.Service, args []string) error {
			if err := s.ConfigUnset(args[0]); err != nil {
				return err
			}
			env.note("%s removed", args[0])
			return nil
		}),
	}
}

func (e Env) printConfig(s *app.Service) error {
	fields := s.ConfigFields()
	if e.jsonMode() {
		return e.writeJSON(fields)
	}
	width := 0
	for _, f := range fields {
		width = max(width, len(f.Key))
	}
	for _, f := range fields {
		line := fmt.Sprintf("%-*s  %s", width, f.Key, f.Value)
		switch {
		case f.Value == "" && f.Derived != "":
			line = fmt.Sprintf("%-*s  %s  (%s)", width, f.Key, f.Derived, f.Source)
		case f.Value != f.Default && f.Default != "":
			line += fmt.Sprintf("  (default %s)", f.Default)
		}
		if _, err := fmt.Fprintln(e.Out, safetext.Line(line)); err != nil {
			return err
		}
	}
	return nil
}
