package cli

import (
	"context"
	"strings"

	"github.com/elliot40404/creds/internal/safetext"
	"github.com/elliot40404/creds/internal/update"
	"github.com/spf13/cobra"
)

func updateCmd(env Env) *cobra.Command {
	var check, yes bool
	c := &cobra.Command{
		Use:   "update",
		Short: "Install the latest creds release",
		Args:  cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			return env.selfUpdate(check, yes)
		},
	}
	c.Flags().BoolVar(&check, "check", false, "only print whether a newer release is out")
	yesFlag(c, &yes)
	return c
}

func (e Env) selfUpdate(check, yes bool) error {
	if _, ok := update.Parse(e.Release); !ok {
		return update.ErrDevBuild
	}
	ctx := context.Background()
	c := e.checker()
	rel, err := c.Latest(ctx)
	if err != nil {
		return err
	}
	latest := strings.TrimPrefix(rel.Tag, "v")
	switch {
	case !update.Newer(rel.Tag, e.Release):
		e.say("creds is up to date (%s)", e.Release)
		return nil
	case check:
		e.say("creds %s is out, this is %s, run creds update", latest, e.Release)
		return nil
	}
	if err := e.confirm(yes, "Update creds "+e.Release+" to "+safetext.Line(latest)+"?"); err != nil {
		return err
	}
	if err := e.install(ctx, c, rel); err != nil {
		return err
	}
	e.say("updated creds %s -> %s", e.Release, latest)
	return nil
}

func (e Env) install(ctx context.Context, c update.Checker, rel update.Release) error {
	path, err := e.exe()
	if err != nil {
		return err
	}
	return c.Install(ctx, rel, path)
}

func (e Env) exe() (string, error) {
	if e.Updates.Exe != nil {
		return e.Updates.Exe()
	}
	return update.Exe()
}
