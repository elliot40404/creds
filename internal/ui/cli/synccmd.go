package cli

import (
	"errors"
	"fmt"
	"time"

	"github.com/elliot40404/creds/internal/app"
	"github.com/elliot40404/creds/internal/fail"
	"github.com/elliot40404/creds/internal/gitsync"
	"github.com/elliot40404/creds/internal/safetext"
	"github.com/elliot40404/creds/internal/vault"
	"github.com/spf13/cobra"
)

var errSide = errors.New("pass exactly one of --mine or --theirs")

func syncCommands(env Env) []*cobra.Command {
	return []*cobra.Command{joinCmd(env), remoteCmd(env), statusCmd(env), syncCmd(env), resolveCmd(env)}
}

func joinCmd(env Env) *cobra.Command {
	var yes bool
	c := &cobra.Command{
		Use:   "join <url>",
		Short: "Clone an existing vault from a git remote",
		Args:  cobra.ExactArgs(1),
		RunE: env.run(func(s *app.Service, args []string) error {
			if err := env.confirm(yes, "Join the vault at "+safetext.Remote(args[0])+"?"); err != nil {
				return err
			}
			if err := s.Join(args[0]); err != nil {
				return err
			}
			env.note("vault joined")
			return nil
		}),
	}
	yesFlag(c, &yes)
	return c
}

func remoteCmd(env Env) *cobra.Command {
	c := &cobra.Command{Use: "remote", Short: "Manage the sync remote", Args: cobra.NoArgs}
	c.AddCommand(&cobra.Command{
		Use:   "add <url>",
		Short: "Set the git remote",
		Args:  cobra.ExactArgs(1),
		RunE: env.run(func(s *app.Service, args []string) error {
			if err := s.RemoteAdd(args[0]); err != nil {
				return err
			}
			env.note("remote added")
			return nil
		}),
	})
	c.AddCommand(simple(env, "remove", "Remove the git remote", (*app.Service).RemoteRemove, "remote removed"))
	return c
}

func statusCmd(env Env) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show sync status",
		Args:  cobra.NoArgs,
		RunE: env.withService(env.Prompter, func(s *app.Service, _ []string) error {
			st, err := s.SyncStatus()
			if err != nil {
				return err
			}
			return env.printStatus(st)
		}),
	}
}

func (e Env) printStatus(st app.SyncStatus) error {
	if e.jsonMode() {
		return e.writeJSON(toJSONStatus(st))
	}
	remote := safetext.Line(safetext.Remote(st.Remote))
	if remote == "" {
		remote = "none"
	}
	last := "never"
	if !st.LastSync.IsZero() {
		last = fmt.Sprintf("%s (%s)", st.LastSync.Local().Format(time.DateTime), safetext.Line(st.LastResult))
	}
	_, _ = fmt.Fprintf(e.Out, "remote: %s\nahead: %d\nbehind: %d\nlast sync: %s\n", remote, st.Ahead, st.Behind, last)
	if st.LastError != "" {
		_, _ = fmt.Fprintf(e.Out, "last error: %s\n", safetext.Text(syncFailure(st.LastError)))
	}
	for _, p := range st.Conflicts {
		_, _ = fmt.Fprintf(e.Out, "conflict: %s\n", safetext.Line(p))
	}
	return nil
}

func syncCmd(env Env) *cobra.Command {
	var quiet bool
	c := &cobra.Command{
		Use:   "sync",
		Short: "Pull and push vault changes",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if quiet {
				return env.withService(noPrompt{}, func(s *app.Service, _ []string) error {
					_ = s.SyncQuiet()
					return nil
				})(cmd, args)
			}
			return env.withService(env.Prompter, func(s *app.Service, _ []string) error {
				s.Waiting = env.waiting
				res, err := s.Sync()
				return env.synced(res, err)
			})(cmd, args)
		},
	}
	c.Flags().BoolVar(&quiet, "quiet", false, "never prompt, record the result only")
	return c
}

func resolveCmd(env Env) *cobra.Command {
	var mine, theirs, yes bool
	c := &cobra.Command{
		Use:   "resolve <path>",
		Short: "Pick a side for a sync conflict",
		Args:  cobra.ExactArgs(1),
		RunE: env.run(func(s *app.Service, args []string) error {
			if mine == theirs {
				return errSide
			}
			side, drop := vault.Theirs, "local"
			if mine {
				side, drop = vault.Mine, "remote"
			}
			if err := env.confirm(yes, "Drop the "+drop+" version of "+args[0]+"?"); err != nil {
				return err
			}
			s.Waiting = env.waiting
			res, err := s.Resolve(args[0], side)
			return env.synced(res, err)
		}),
	}
	c.Flags().BoolVar(&mine, "mine", false, "keep the local version")
	c.Flags().BoolVar(&theirs, "theirs", false, "keep the remote version")
	yesFlag(c, &yes)
	return c
}

func (e Env) waiting() {
	e.note("waiting for another sync...")
}

func (e Env) synced(res string, err error) error {
	if cerr, ok := errors.AsType[*app.ConflictError](err); ok {
		if e.jsonMode() {
			_ = e.writeJSON(jsonSync{Result: "conflict", Conflicts: nonNil(cerr.Paths)})
		} else {
			for _, p := range cerr.Paths {
				e.note("conflict: %s", p)
			}
		}
		return fmt.Errorf("%w: %w", gitsync.ErrConflict, err)
	}
	if err != nil {
		return err
	}
	if e.jsonMode() {
		return e.writeJSON(jsonSync{Result: res, Conflicts: []string{}})
	}
	e.note("sync: %s", res)
	return nil
}

func syncFailure(raw string) string {
	msg, hint := fail.SyncText(raw)
	if hint == "" {
		hint = "run creds sync"
	}
	return msg + ", " + hint
}

func plainSyncError(raw string) string {
	msg, _ := fail.SyncText(raw)
	return msg
}
