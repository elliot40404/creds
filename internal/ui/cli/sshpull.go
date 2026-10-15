package cli

import (
	"github.com/elliot40404/creds/internal/app"
	"github.com/elliot40404/creds/internal/safetext"
	"github.com/spf13/cobra"
)

const sshPullLong = `Write ssh files from the vault into ~/.ssh.

Files missing here are created and files changed only in the vault are
updated. A file changed here and in the vault shows a diff and is overwritten
only with -y or a yes at the prompt. A file deleted in the vault is deleted
here only after a yes at the prompt, -y never deletes. Files are written for
your account only, ~/.ssh is created if missing.`

var sshPullHints = []sshHint{
	{app.SSHKeptConflict, "%d files changed here and in the vault were not written, run creds ssh pull -y to overwrite the local copy"},
	{app.SSHNotAsked, "%d files deleted in the vault are still here, run creds ssh pull in a terminal to delete them"},
	{app.SSHNeedsPush, "%d files changed here, run creds ssh push"},
}

func sshPullCmd(env Env) *cobra.Command {
	var o app.SSHPullOptions
	c := &cobra.Command{
		Use:   "pull",
		Short: "Write new and changed vault ssh files into ~/.ssh",
		Long:  sshPullLong,
		Args:  cobra.NoArgs,
		RunE: env.run(func(s *app.Service, _ []string) error {
			res, err := s.SSHPull(env.sshPullAsk(o))
			if err != nil {
				return err
			}
			return env.printSSHPull(res)
		}),
	}
	c.Flags().BoolVarP(&o.Overwrite, "yes", "y", false, "overwrite local files that changed here and in the vault")
	return c
}

func (e Env) sshPullAsk(o app.SSHPullOptions) app.SSHPullOptions {
	if !isTTY(e.Prompter) {
		return o
	}
	o.AskConflict = func(f app.SSHFileStatus) (bool, error) {
		e.sshDiff(f)
		return e.Prompter.Confirm(safetext.Line(f.Rel) + " changed here and in the vault. Overwrite the local copy?")
	}
	o.AskDelete = func(f app.SSHFileStatus) (bool, error) {
		q := "Delete " + safetext.Line(f.Rel) + "? It was deleted in the vault"
		if f.Change == app.SSHConflict {
			q += " and changed here"
		}
		return e.Prompter.Confirm(q + ".")
	}
	return o
}

func (e Env) printSSHPull(res app.SSHPullResult) error {
	counts, err := e.printSSHFiles(res.Status, res.Files, sshActionColumn, sshPullHints)
	if err != nil || e.jsonMode() {
		return err
	}
	wrote := counts[app.SSHCreated] + counts[app.SSHUpdated] + counts[app.SSHDeleted]
	if wrote == 0 && counts[app.SSHFailed] == 0 {
		e.note("nothing to pull")
		return nil
	}
	e.note("pulled %d files", wrote)
	return nil
}
