package cli

import (
	"context"
	"os"

	"github.com/elliot40404/creds/internal/app"
	"github.com/elliot40404/creds/internal/config"
	"github.com/elliot40404/creds/internal/detach"
	"github.com/elliot40404/creds/internal/update"
	"github.com/spf13/cobra"
)

const updateCheckName = "update-check"

var noUpdateNoteCmds = []string{"update", "version"}

type Updates struct {
	Spawn func() error
	API   string
}

func spawnUpdateCheck() error {
	return detach.Self(os.Environ(), updateCheckName)
}

func (e Env) updateNotes(cmd *cobra.Command, n app.Notice) {
	if under(cmd, noUpdateNoteCmds) {
		return
	}
	if n.Update != "" {
		e.note("creds %s is out, run creds update", n.Update)
	}
	if n.CheckUpdate && e.Updates.Spawn != nil {
		_ = e.Updates.Spawn()
	}
}

func updateCheckCmd(env Env) *cobra.Command {
	return &cobra.Command{
		Use:    updateCheckName,
		Short:  "Check for a new creds release in the background",
		Hidden: true,
		Args:   cobra.NoArgs,
		RunE: env.withService(noPrompt{}, func(s *app.Service, _ []string) error {
			if s.CurrentConfig().Update.Current() != config.UpdateOff {
				_ = s.CheckUpdate(context.Background(), env.checker())
			}
			return nil
		}),
	}
}

func (e Env) checker() update.Checker {
	c := update.New(e.Release)
	if e.Updates.API != "" {
		c.BaseURL = e.Updates.API
	}
	return c
}
