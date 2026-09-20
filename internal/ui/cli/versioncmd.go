package cli

import (
	"fmt"

	"github.com/elliot40404/creds/internal/buildinfo"
	"github.com/spf13/cobra"
)

func versionCmd(env Env) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Show the creds version",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			return env.printVersion()
		},
	}
}

func (e Env) printVersion() error {
	info := buildinfo.Get()
	if e.jsonMode() {
		return e.writeJSON(info)
	}
	_, err := fmt.Fprintln(e.Out, info)
	return err
}
