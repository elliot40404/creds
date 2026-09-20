package cli

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/elliot40404/creds/internal/app"
	"github.com/elliot40404/creds/internal/envfile"
	"github.com/elliot40404/creds/internal/fsutil"
	"github.com/spf13/cobra"
)

func envCommands(env Env) []*cobra.Command {
	return []*cobra.Command{importEnvCmd(env), envCmd(env), runCmd(env), trustCmd(env)}
}

func importEnvCmd(env Env) *cobra.Command {
	return &cobra.Command{
		Use:   "import-env <file> <path>",
		Short: "Import a .env file as one entry",
		Args:  cobra.ExactArgs(2),
		RunE: env.run(func(s *app.Service, args []string) error {
			f, err := os.Open(filepath.Clean(args[0]))
			if err != nil {
				return err
			}
			defer func() { _ = f.Close() }()
			warns, err := s.ImportEnv(f, args[1])
			if err != nil {
				return err
			}
			for _, w := range warns {
				env.note("warning: %s", w)
			}
			env.note("imported %s", args[1])
			return nil
		}),
	}
}

var errEnvFormat = errors.New("unknown env format")

const (
	envDotenv = "dotenv"
	envShell  = "sh"
)

func envCmd(env Env) *cobra.Command {
	var out, format string
	var yes bool
	c := &cobra.Command{
		Use:               "env [path]",
		Short:             "Print env vars in .env format",
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: env.completePaths,
		RunE: env.run(func(s *app.Service, args []string) error {
			write, err := envWriter(format)
			if err != nil {
				return err
			}
			vars, err := projectVars(s, args)
			if err != nil {
				return err
			}
			if out == "" && env.jsonMode() {
				return env.writeJSON(toJSONEnv(vars))
			}
			var b bytes.Buffer
			if err := write(&b, vars); err != nil {
				return err
			}
			if out == "" {
				return env.writeRaw(b.String())
			}
			return env.writeSecretFile(out, b.Bytes(), yes)
		}),
	}
	c.Flags().StringVarP(&out, "output", "o", "", "write to this file instead of stdout")
	c.Flags().StringVar(&format, "format", envDotenv, "dotenv (node, python, docker) or sh (source it in a shell)")
	yesFlag(c, &yes)
	return c
}

func envWriter(format string) (func(io.Writer, []envfile.Var) error, error) {
	switch format {
	case envDotenv:
		return envfile.WriteDotenv, nil
	case envShell:
		return envfile.Write, nil
	}
	return nil, fmt.Errorf("%w %q", errEnvFormat, format)
}

func projectVars(s *app.Service, args []string) ([]envfile.Var, error) {
	dir, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	path := ""
	if len(args) > 0 {
		path = args[0]
	}
	return s.ProjectEnv(dir, path)
}

func (e Env) writeSecretFile(path string, data []byte, yes bool) error {
	if err := e.confirm(yes, "Write secrets to "+path+"?"); err != nil {
		return err
	}
	if err := fsutil.WriteFileAtomic(path, data); err != nil {
		return err
	}
	e.note("wrote %s", path)
	return nil
}

func (e Env) confirm(yes bool, prompt string) error {
	if yes {
		return nil
	}
	ok, err := e.Prompter.Confirm(prompt)
	if err != nil {
		return err
	}
	if !ok {
		return app.ErrAborted
	}
	return nil
}

func yesFlag(c *cobra.Command, yes *bool) {
	c.Flags().BoolVarP(yes, "yes", "y", false, "skip confirmation")
}
