package cli

import (
	"github.com/elliot40404/creds/internal/app"
	"github.com/spf13/cobra"
)

var noAutoSSHCmds = []string{"ssh", "lock", "config"}

func (e Env) autoSSH(cmd *cobra.Command, s *app.Service) {
	if under(cmd, noAutoSSHCmds) {
		return
	}
	e.sshHint(s.SSH().Auto())
}

func (e Env) sshHint(hint string) {
	if hint != "" {
		e.note("%s", hint)
	}
}
