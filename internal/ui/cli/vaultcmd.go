package cli

import (
	"github.com/elliot40404/creds/internal/app"
	"github.com/spf13/cobra"
)

func vaultCommands(env Env) []*cobra.Command {
	return []*cobra.Command{
		simple(env, "init", "Create a new vault", (*app.Service).Init, "vault created, next run creds add -i to add your first entry"),
		simple(env, "unlock", "Unlock the vault for this session", (*app.Service).Unlock, "unlocked"),
		simple(env, "lock", "End the current session", (*app.Service).Lock, "locked"),
		simple(env, "recover", "Set a new master password using the recovery code", (*app.Service).Recover, "master password replaced"),
		simple(env, "passwd", "Change the master password", (*app.Service).Passwd, "master password changed"),
	}
}

func simple(env Env, use, short string, fn func(*app.Service) error, done string) *cobra.Command {
	return &cobra.Command{
		Use:   use,
		Short: short,
		Args:  cobra.NoArgs,
		RunE: env.run(func(s *app.Service, _ []string) error {
			if err := fn(s); err != nil {
				return err
			}
			env.note("%s", done)
			return nil
		}),
	}
}
