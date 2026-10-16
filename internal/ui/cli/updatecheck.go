package cli

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"time"

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
	HTTP  *http.Client
	Exe   func() (string, error)
}

func spawnUpdateCheck() error {
	return detach.Self(os.Environ(), updateCheckName)
}

func (e Env) updateNotes(cmd *cobra.Command, n app.Notice) {
	if n.UpdateFailed {
		e.note("creds update failed in the background, run creds update")
	}
	if n.Updated != "" {
		e.note("creds updated to %s", n.Updated)
	}
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
			if mode := s.CurrentConfig().Update.Current(); mode != config.UpdateOff {
				env.backgroundUpdate(s, mode)
			}
			return nil
		}),
	}
}

func (e Env) backgroundUpdate(s *app.Service, mode config.UpdateMode) {
	ctx, c := context.Background(), e.checker()
	rel, err := s.CheckUpdate(ctx, c)
	exe, ok := e.releaseExe()
	if err != nil || !ok || mode != config.UpdateInstall || !update.Newer(rel.Tag, e.Release) {
		return
	}
	_ = s.InstallUpdate(rel.Tag, func() error { return c.Install(ctx, rel, exe) })
}

func (e Env) releaseExe() (string, bool) {
	if _, ok := update.Parse(e.Release); !ok {
		return "", false
	}
	exe, err := e.exe()
	if err != nil {
		return "", false
	}
	update.RemoveStale(filepath.Dir(exe), time.Now())
	return exe, true
}

func (e Env) checker() update.Checker {
	c := update.New(e.Release)
	if e.Updates.API != "" {
		c.BaseURL = e.Updates.API
	}
	if e.Updates.HTTP != nil {
		c.Client, c.Download = e.Updates.HTTP, e.Updates.HTTP
	}
	return c
}
