package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"time"

	"github.com/elliot40404/creds/internal/app"
	"github.com/elliot40404/creds/internal/buildinfo"
	"github.com/elliot40404/creds/internal/config"
	"github.com/elliot40404/creds/internal/fail"
	"github.com/elliot40404/creds/internal/safetext"
	"github.com/spf13/cobra"
)

var noConfigCmds = []string{"lock", "paths", "config"}

type Env struct {
	In       io.Reader
	Out      io.Writer
	Err      io.Writer
	Prompter app.Prompter
	Now      func() time.Time
	LogN     int
	Clip     Clip
	Spawn    func() error
	Home     string
	Setup    func(*app.Setup)
	Editor   func(path string) error
	json     *bool
}

func Main(args []string) int {
	env := Env{In: os.Stdin, Out: os.Stdout, Err: os.Stderr, Prompter: NewTerminal(os.Stdin, os.Stderr), Clip: SystemClip(), Spawn: spawnSync}
	return Run(env, args)
}

func Run(env Env, args []string) int {
	env.json = new(bool)
	if cmd, ok := env.interactiveCmd(args); ok {
		return env.interview(cmd, args)
	}
	return env.execute(args)
}

func (e Env) execute(args []string) int {
	root := NewRoot(e)
	root.SetArgs(args)
	cmd, err := root.ExecuteC()
	return e.report(cmd, args, err)
}

func (e Env) report(cmd *cobra.Command, args []string, err error) int {
	if err == nil {
		return 0
	}
	if code, ok := errors.AsType[exitError](err); ok {
		return int(code)
	}
	fe := fillHint(cmd, args, classify(cmd, err))
	if e.jsonMode() || wantJSON(args) {
		_, _ = fmt.Fprintf(e.Err, "%s\n", fail.JSON(fe))
	} else {
		_, _ = io.WriteString(e.Err, fail.Text(fe))
	}
	return int(fe.Code)
}

func NewRoot(env Env) *cobra.Command {
	root := &cobra.Command{
		Use:           "creds",
		Short:         "Local encrypted credential vault",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	if env.json == nil {
		env.json = new(bool)
	}
	root.PersistentFlags().BoolVar(env.json, flagJSON, false, "print data as JSON on stdout and errors as JSON on stderr")
	root.PersistentFlags().BoolP(flagInteractive, "i", false, "ask for missing arguments and flags")
	root.Version = buildinfo.Get().String()
	root.SetVersionTemplate("{{.Version}}\n")
	root.SetOut(env.Out)
	root.SetErr(env.Err)
	root.AddCommand(vaultCommands(env)...)
	root.AddCommand(entryCommands(env)...)
	root.AddCommand(envCommands(env)...)
	root.AddCommand(syncCommands(env)...)
	root.AddCommand(exportCommands(env)...)
	root.AddCommand(clipClearCmd(env))
	root.AddCommand(agentsTopic())
	root.AddCommand(configCommands(env)...)
	root.AddCommand(setupCmd(env))
	root.AddCommand(pathsCmd(env))
	root.AddCommand(versionCmd(env))
	root.AddCommand(pickCmd(env, root))
	setupRoot(env, root)
	markRun(root)
	return root
}

func (e Env) service(p app.Prompter) (*app.Service, error) {
	return e.serviceFor(nil, p)
}

func (e Env) serviceFor(cmd *cobra.Command, p app.Prompter) (*app.Service, error) {
	paths, err := e.paths()
	if err != nil {
		return nil, err
	}
	cfg, err := config.Load(paths.Config())
	if err != nil {
		if !configOptional(cmd) {
			return nil, err
		}
		e.note("warning: %v", err)
		e.note("warning: using default settings, fix %s with creds config edit", paths.Config())
		cfg = config.Default()
	}
	return &app.Service{Paths: paths, Config: cfg, Prompter: p, Now: e.Now, LogN: e.LogN, Spawn: e.Spawn}, nil
}

func configOptional(cmd *cobra.Command) bool {
	for c := cmd; c != nil; c = c.Parent() {
		if slices.Contains(noConfigCmds, c.Name()) {
			return true
		}
	}
	return false
}

func (e Env) paths() (config.Paths, error) {
	if e.Home != "" {
		return config.Paths{Home: e.Home}, nil
	}
	return config.DefaultPaths()
}

func (e Env) run(fn func(*app.Service, []string) error) func(*cobra.Command, []string) error {
	return e.withService(e.Prompter, func(s *app.Service, args []string) error {
		if msg := s.SyncFailure(); msg != "" {
			e.note("last sync failed: %s", syncFailure(msg))
		}
		if msg := s.TakeClearFailure(); msg != "" {
			e.note("warning: clipboard was not cleared after the last copy: %s", msg)
		}
		return fn(s, args)
	})
}

func (e Env) withService(p app.Prompter, fn func(*app.Service, []string) error) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		s, err := e.serviceFor(cmd, p)
		if err != nil {
			return err
		}
		defer e.warn(s)
		return fn(s, args)
	}
}

func (e Env) warn(s *app.Service) {
	for _, w := range s.Warnings() {
		e.note("warning: %s", w)
	}
}

func (e Env) note(format string, a ...any) {
	msg := safetext.Text(fmt.Sprintf(format, a...))
	if e.jsonMode() {
		_ = writeJSONLine(e.Err, jsonNote{Note: msg})
		return
	}
	_, _ = fmt.Fprintln(e.Err, msg)
}

func wantJSON(args []string) bool {
	if i := slices.Index(args, "--"); i >= 0 {
		args = args[:i]
	}
	return slices.Contains(args, "--json")
}
