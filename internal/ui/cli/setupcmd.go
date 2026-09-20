package cli

import (
	"fmt"

	"github.com/elliot40404/creds/internal/app"
	"github.com/elliot40404/creds/internal/gh"
	"github.com/elliot40404/creds/internal/safetext"
	"github.com/spf13/cobra"
)

func setupCmd(env Env) *cobra.Command {
	return &cobra.Command{
		Use:   "setup",
		Short: "Guided first run setup",
		Args:  cobra.NoArgs,
		RunE: env.withService(env.Prompter, func(s *app.Service, _ []string) error {
			return env.setup(s)
		}),
	}
}

func setupRoot(env Env, root *cobra.Command) {
	next := root.RunE
	root.RunE = func(cmd *cobra.Command, args []string) error {
		return env.withService(env.Prompter, func(s *app.Service, _ []string) error {
			ok, err := s.HasVault()
			if err != nil {
				return err
			}
			if ok {
				return next(cmd, args)
			}
			return env.setup(s)
		})(cmd, args)
	}
}

func (e Env) setup(s *app.Service) error {
	u := s.Setup()
	u.Look = gh.Look
	u.Host = gh.New()
	if e.Setup != nil {
		e.Setup(u)
	}
	if !isTTY(e.Prompter) {
		return e.printSteps(u)
	}
	return e.setupUI(u)
}

func (e Env) printSteps(u *app.Setup) error {
	has, err := u.Service.HasVault()
	if err != nil {
		return err
	}
	if has {
		e.printManageSteps(u)
		return nil
	}
	e.say("creds is not set up yet, run these in a terminal:")
	e.say("  creds setup      guided setup, or run the steps below yourself")
	e.say("  creds init       create a new vault")
	e.say("  creds remote add <url>   push it to an empty git remote")
	e.say("  creds join <url>         use a vault that already exists")
	e.sayChecks(u.Preflight())
	return nil
}

func (e Env) printManageSteps(u *app.Setup) {
	e.say("this home already has a vault, run these in a terminal:")
	e.say("  creds setup              change the remote or the settings")
	e.say("  creds remote add <url>   push it to an empty git remote")
	e.sayChecks(u.Preflight())
	e.nextSteps(u)
}

func (e Env) sayChecks(checks []app.Check) {
	for _, c := range checks {
		if c.OK {
			e.say("  ok   %s: %s", c.Name, c.Message)
			continue
		}
		e.say("  fix  %s: %s", c.Name, c.Message)
		if c.Fix != "" {
			e.say("       %s", c.Fix)
		}
	}
}

func (e Env) say(format string, a ...any) {
	_, _ = fmt.Fprintln(e.Out, safetext.Text(fmt.Sprintf(format, a...)))
}

func (e Env) nextSteps(u *app.Setup) {
	e.say("setup done, next:")
	for _, step := range u.NextSteps() {
		e.say("  %s", step)
	}
}
