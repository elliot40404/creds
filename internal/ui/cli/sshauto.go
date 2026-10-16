package cli

import (
	"github.com/elliot40404/creds/internal/app"
	"github.com/spf13/cobra"
)

var noAutoPushCmds = []string{"ssh", "lock", "config"}

func (e Env) autoPush(cmd *cobra.Command, s *app.Service) {
	if under(cmd, noAutoPushCmds) {
		return
	}
	if hint := s.SSHAutoPush(); hint != "" {
		e.note("%s", hint)
	}
}
