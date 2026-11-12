package cli

import (
	"github.com/elliot40404/creds/internal/app"
	"github.com/spf13/cobra"
)

const sshStatusLong = `Show how ~/.ssh files differ from the vault, changes nothing.

Each file that differs is listed with its change: local new, local changed,
local deleted, remote new, remote changed, remote deleted or conflict. Local
means here, remote means the vault. The hints say whether to run
creds ssh push or creds ssh pull.`

var sshChangeColumn = sshColumn{14, func(f app.SSHFileResult) string { return f.Change.String() }}

var sshStatusHints = []sshHint{
	{app.SSHNeedsPush, "%d files changed here, run creds ssh push"},
	{app.SSHNeedsPull, "%d files changed in the vault, run creds ssh pull"},
	{app.SSHKeptConflict, "%d files changed here and in the vault, run creds ssh pull to see the diff"},
	{app.SSHKeptDelete, "%d files deleted here are still in the vault"},
	{app.SSHSkippedKey, "%d unused keys skipped, run creds ssh push --all to push them"},
}

func sshStatusCmd(env Env) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show how ~/.ssh files differ from the vault",
		Long:  sshStatusLong,
		Args:  cobra.NoArgs,
		RunE: env.run(func(s *app.Service, _ []string) error {
			st, err := s.SSH().Status()
			if err != nil {
				return err
			}
			return env.printSSHStatus(st)
		}),
	}
}

func (e Env) printSSHStatus(st app.SSHStatus) error {
	files := make([]app.SSHFileResult, len(st.Files))
	for i, f := range st.Files {
		files[i] = app.SSHFileResult{Rel: f.Rel, Change: f.Change, Action: f.Next()}
	}
	counts, err := e.printSSHFiles(st, files, sshChangeColumn, sshStatusHints)
	if err != nil || e.jsonMode() {
		return err
	}
	switch differ := len(files) - counts[app.SSHNoChange]; {
	case len(files) == 0:
		e.note("no ssh files here or in the vault")
	case differ == 0:
		e.note("%d files up to date", len(files))
	default:
		e.note("%d of %d files differ", differ, len(files))
	}
	return nil
}
