package cli

import (
	"github.com/elliot40404/creds/internal/app"
	"github.com/elliot40404/creds/internal/safetext"
	"github.com/spf13/cobra"
)

const sshPushLong = `Copy ~/.ssh files into the vault: config, config.d, private keys and .pub files.

New and changed files are pushed in one vault change. Keys that no ssh config
uses are asked about once, the answer is saved. Without a terminal they are
left out and listed, --all pushes them. A file changed here and in the vault
is pushed only with -y or a yes at the prompt. A file deleted here stays in
the vault.`

func sshCmd(env Env) *cobra.Command {
	c := &cobra.Command{Use: "ssh", Short: "Back up ~/.ssh files in the vault", Args: cobra.NoArgs}
	c.AddCommand(sshPushCmd(env), sshPullCmd(env), sshStatusCmd(env), sshAddHostCmd(env), sshAddKeyCmd(env), sshKeygenCmd(env))
	return c
}

func sshPushCmd(env Env) *cobra.Command {
	var o app.SSHPushOptions
	c := &cobra.Command{
		Use:   "push",
		Short: "Copy new and changed ~/.ssh files into the vault",
		Long:  sshPushLong,
		Args:  cobra.NoArgs,
		RunE: env.run(func(s *app.Service, _ []string) error {
			res, err := s.SSH().Push(env.sshAsk(o))
			if err != nil {
				return err
			}
			return env.printSSHPush(res)
		}),
	}
	c.Flags().BoolVar(&o.All, "all", false, "push every private key, also keys no ssh config uses")
	c.Flags().BoolVarP(&o.Overwrite, "yes", "y", false, "overwrite vault copies that changed here and in the vault")
	return c
}

func (e Env) sshAsk(o app.SSHPushOptions) app.SSHPushOptions {
	if !isTTY(e.Prompter) {
		return o
	}
	o.AskKey = func(rel string) (bool, error) {
		return e.Prompter.Confirm("Push " + safetext.Line(rel) + ", a key no ssh config uses?")
	}
	o.AskConflict = func(rel string) (bool, error) {
		return e.Prompter.Confirm(safetext.Line(rel) + " changed here and in the vault. Overwrite the vault copy?")
	}
	return o
}
