package cli

import (
	"fmt"

	"github.com/elliot40404/creds/internal/app"
	"github.com/elliot40404/creds/internal/envfile"
	"github.com/elliot40404/creds/internal/safetext"
	"github.com/spf13/cobra"
)

func trustCmd(env Env) *cobra.Command {
	var yes bool
	c := &cobra.Command{
		Use:   "trust [dir]",
		Short: "Allow a project .creds.toml to pick vault secrets",
		Args:  cobra.MaximumNArgs(1),
		RunE: env.withService(env.Prompter, func(s *app.Service, args []string) error {
			dir := "."
			if len(args) > 0 {
				dir = args[0]
			}
			p, err := envfile.LoadProject(dir)
			if err != nil {
				return err
			}
			if err := env.printRefs(p); err != nil {
				return err
			}
			if err := env.confirm(yes, "Trust "+p.File+" to pick these secrets?"); err != nil {
				return err
			}
			if err := s.TrustProject(p); err != nil {
				return err
			}
			env.note("trusted %s", p.File)
			return nil
		}),
	}
	yesFlag(c, &yes)
	c.AddCommand(trustListCmd(env), trustRemoveCmd(env))
	return c
}

func trustListCmd(env Env) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List trusted project files",
		Args:  cobra.NoArgs,
		RunE: env.withService(noPrompt{}, func(s *app.Service, _ []string) error {
			files, err := s.TrustList()
			if err != nil {
				return err
			}
			if env.jsonMode() {
				return env.writeJSON(map[string][]string{"trusted": nonNil(files)})
			}
			for _, f := range files {
				if _, err := fmt.Fprintln(env.Out, safetext.Line(f)); err != nil {
					return err
				}
			}
			return nil
		}),
	}
}

func trustRemoveCmd(env Env) *cobra.Command {
	var yes bool
	c := &cobra.Command{
		Use:   "remove <file>",
		Short: "Forget a trusted project file",
		Args:  cobra.ExactArgs(1),
		RunE: env.withService(env.Prompter, func(s *app.Service, args []string) error {
			if err := env.confirm(yes, "Forget "+args[0]+"?"); err != nil {
				return err
			}
			if err := s.Untrust(args[0]); err != nil {
				return err
			}
			env.note("forgot %s", args[0])
			return nil
		}),
	}
	yesFlag(c, &yes)
	return c
}

func (e Env) printRefs(p envfile.Project) error {
	if e.jsonMode() {
		return e.writeJSON(toJSONTrust(p))
	}
	for _, r := range p.Refs() {
		if _, err := fmt.Fprintln(e.Out, safetext.Line(r)); err != nil {
			return err
		}
	}
	return nil
}
