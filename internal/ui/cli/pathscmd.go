package cli

import (
	"fmt"

	"github.com/elliot40404/creds/internal/app"
	"github.com/elliot40404/creds/internal/config"
	"github.com/elliot40404/creds/internal/safetext"
	"github.com/spf13/cobra"
)

func pathsCmd(env Env) *cobra.Command {
	return &cobra.Command{
		Use:   "paths",
		Short: "Show where creds keeps its files",
		Args:  cobra.NoArgs,
		RunE: env.withService(noPrompt{}, func(s *app.Service, _ []string) error {
			return env.printPaths(s.PathReport())
		}),
	}
}

func (e Env) printPaths(r app.PathReport) error {
	if e.jsonMode() {
		return e.writeJSON(r)
	}
	name, path := 0, 0
	for _, p := range r.Paths {
		name = max(name, len(p.Name))
		path = max(path, len(p.Path))
	}
	for _, p := range r.Paths {
		line := fmt.Sprintf("%-*s  %-*s  %s", name, p.Name, path, p.Path, state(p.Exists))
		if p.Note != "" {
			line += "  " + p.Note
		}
		if _, err := fmt.Fprintln(e.Out, safetext.Line(line)); err != nil {
			return err
		}
	}
	if r.HomeEnv {
		e.note("%s is set, creds uses that home", config.HomeEnv)
	}
	return nil
}

func state(exists bool) string {
	if exists {
		return "exists"
	}
	return "missing"
}
