package cli

import (
	"errors"
	"fmt"

	"github.com/elliot40404/creds/internal/runner"
	"github.com/spf13/cobra"
)

var (
	ErrNeedDash  = errors.New("missing -- before the command")
	errNoProgram = errors.New("program not found")
)

type exitError int

func (e exitError) Error() string {
	return fmt.Sprintf("command exited with code %d", int(e))
}

func runCmd(env Env) *cobra.Command {
	var lock bool
	cmd := &cobra.Command{
		Use:               "run [path] -- <command> [args...]",
		Short:             "Run a command with env vars injected",
		ValidArgsFunction: env.completePaths,
		Args: func(cmd *cobra.Command, args []string) error {
			dash := cmd.ArgsLenAtDash()
			if dash < 0 || dash > 1 || dash == len(args) {
				return ErrNeedDash
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			dash := cmd.ArgsLenAtDash()
			return env.runWithVars(args[:dash], args[dash:], lock)
		},
	}
	cmd.Flags().BoolVar(&lock, "lock", false, "end the session before the command starts")
	return cmd
}

func (e Env) runWithVars(pathArgs, command []string, lock bool) error {
	s, err := e.service(e.Prompter)
	if err != nil {
		return err
	}
	vars, err := projectVars(s, pathArgs)
	e.warn(s)
	if err != nil {
		return err
	}
	if lock {
		if err := s.Lock(); err != nil {
			return err
		}
	}
	pairs := make([]string, len(vars))
	for i, v := range vars {
		pairs[i] = v.Key + "=" + v.Value
	}
	code, err := runner.Run(runner.Cmd{Args: command, Env: pairs, Stdin: e.In, Stdout: e.Out, Stderr: e.Err})
	if runner.IsNotFound(err) {
		return fmt.Errorf("%s: %w", command[0], errNoProgram)
	}
	if err != nil {
		return err
	}
	if code != 0 {
		return exitError(code)
	}
	return nil
}
