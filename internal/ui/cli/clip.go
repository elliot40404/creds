package cli

import (
	"errors"
	"io"
	"os"
	"runtime"
	"time"

	"github.com/elliot40404/creds/internal/app"
	"github.com/elliot40404/creds/internal/clipboard"
	"github.com/elliot40404/creds/internal/fail"
	"github.com/elliot40404/creds/internal/render"
	"github.com/elliot40404/creds/internal/ui/tui"
	"github.com/elliot40404/creds/internal/vault"
	"github.com/spf13/cobra"
)

var errNoOSC52Terminal = errors.New("no terminal for OSC52")

type Clip struct {
	In      io.Reader
	Native  clipboard.Native
	Getenv  func(string) string
	OpenTTY func() (*os.File, error)
	IsTTY   func(any) bool
	Spawn   func(clipboard.ClearJob) error
	Sleep   func(time.Duration)
}

func SystemClip() Clip {
	return Clip{
		In:      os.Stdin,
		Native:  clipboard.System{},
		Getenv:  os.Getenv,
		OpenTTY: clipboard.OpenTTY,
		IsTTY:   isTerminal,
		Spawn:   clipboard.Spawn,
		Sleep:   time.Sleep,
	}
}

func (c Clip) tmux() bool {
	return c.Getenv("TMUX") != ""
}

func clipClearCmd(env Env) *cobra.Command {
	var after time.Duration
	var mode string
	cmd := &cobra.Command{
		Use:    clipboard.ClearCommand,
		Hidden: true,
		Args:   cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			err := env.clearClip(after, mode)
			if err != nil {
				env.recordClearFailure(err)
			}
			return err
		},
	}
	cmd.Flags().DurationVar(&after, "after", 0, "delay before clearing")
	cmd.Flags().StringVar(&mode, "mode", "", "clipboard mode")
	return cmd
}

func (e Env) clearClip(after time.Duration, mode string) error {
	m, err := clipboard.ParseMode(mode)
	if err != nil {
		return err
	}
	run := clipboard.ClearRun{In: e.Clip.In, After: after, Mode: m, Native: e.Clip.Native, Sleep: e.Clip.Sleep}
	if m == clipboard.ModeOSC52 {
		tty := clipboard.ChildTTY()
		if tty == nil {
			return clipboard.ErrNoTTY
		}
		defer func() { _ = tty.Close() }()
		run.Terminal = clipboard.Terminal{W: tty, Tmux: e.Clip.tmux()}
	}
	return run.Run()
}

func (e Env) recordClearFailure(err error) {
	paths, perr := e.paths()
	if perr != nil {
		return
	}
	s := app.Service{Paths: paths}
	_ = s.RecordClearFailure(err)
}

func copyCmd(env Env) *cobra.Command {
	var field, as, shell string
	var osc52, native bool
	c := &cobra.Command{
		Use:               "copy <path>",
		Aliases:           []string{"cp"},
		Short:             "Copy a secret to the clipboard",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: env.completePaths,
		RunE: env.run(func(s *app.Service, args []string) error {
			sh, err := renderShell(shell, as)
			if err != nil {
				return err
			}
			label, warn, err := env.copyEntry(s, args[0], field, as, sh, forcedMode(osc52, native))
			if err != nil {
				return env.offerValue(err)
			}
			env.note("Copied %s", label)
			if warn != nil {
				env.note("clipboard will not be cleared automatically: %v", warn)
			}
			return nil
		}),
	}
	c.Flags().StringVar(&field, "field", "", "field to copy")
	c.Flags().StringVar(&as, "as", "", "copy a database entry in this format")
	c.Flags().StringVar(&shell, "shell", "", "quote for this shell instead of render.shell, needs --as")
	c.Flags().BoolVar(&osc52, "osc52", false, "copy through the terminal (OSC52)")
	c.Flags().BoolVar(&native, "native", false, "copy to the system clipboard")
	c.MarkFlagsMutuallyExclusive("osc52", "native")
	c.MarkFlagsMutuallyExclusive("field", "as")
	return c
}

func (e Env) copyEntry(s *app.Service, path, field, as string, sh render.Shell, forced clipboard.Mode) (label string, warn, err error) {
	value, label, hint, err := copySource(s, path, field, as, sh)
	if err != nil {
		return "", nil, err
	}
	mode := clipboard.Resolve(forced, e.Clip.Getenv, runtime.GOOS)
	warn, err = e.copyValue(mode, value, s.CurrentConfig().Clipboard.Clear)
	if err != nil {
		return "", nil, copyFailure(path+" "+hint, &tui.CopyFailed{Label: label, Value: value, Err: err})
	}
	return label, warn, nil
}

func copyFailure(get string, cause *tui.CopyFailed) error {
	hint := "run creds get " + get
	if errors.Is(cause.Err, errNoOSC52Terminal) {
		hint = "run creds copy --native, or creds get " + get
	}
	return &fail.Error{Msg: cause.Err.Error(), Hint: hint, Code: fail.General, Err: cause}
}

func (e Env) offerValue(err error) error {
	cf, ok := errors.AsType[*tui.CopyFailed](err)
	if !ok || e.jsonMode() || !e.Clip.IsTTY(e.In) || !e.Clip.IsTTY(e.Out) {
		return err
	}
	yes, perr := e.Prompter.Confirm("Clipboard failed (" + cf.Err.Error() + "). Show " + cf.Label + " here to copy by hand?")
	if perr != nil || !yes {
		return err
	}
	if werr := e.writeRaw(cf.Value + "\n"); werr != nil {
		return werr
	}
	e.note("Shown %s, clipboard not used", cf.Label)
	return nil
}

func copySource(s *app.Service, path, field, as string, sh render.Shell) (value, label, hint string, err error) {
	if as != "" {
		value, err = s.Render(path, as, sh)
		return value, path + " as " + as + shellLabel(sh), "--as " + as + shellHint(sh), err
	}
	e, err := s.Get(path)
	if err != nil {
		return "", "", "", err
	}
	if field == "" && e.Type == vault.TypeDatabase {
		value, err = s.Render(path, render.FormatURL, "")
		return value, path + " as " + render.FormatURL, "--as " + render.FormatURL, err
	}
	if field == "" {
		if field, err = app.DefaultField(e); err != nil {
			return "", "", "", err
		}
	}
	value, err = app.Lookup(e, field)
	return value, e.Path + "." + field, "--field " + field + " --show", err
}

func shellLabel(sh render.Shell) string {
	if sh == "" {
		return ""
	}
	return " in " + string(sh)
}

func shellHint(sh render.Shell) string {
	if sh == "" {
		return ""
	}
	return " --shell " + string(sh)
}

func forcedMode(osc52, native bool) clipboard.Mode {
	switch {
	case osc52:
		return clipboard.ModeOSC52
	case native:
		return clipboard.ModeNative
	}
	return clipboard.ModeAuto
}

func (e Env) copyValue(mode clipboard.Mode, value string, after time.Duration) (warn, err error) {
	job := clipboard.ClearJob{After: after, Mode: mode, Hash: clipboard.Hash(value)}
	if mode == clipboard.ModeNative {
		if err := e.Clip.Native.Write(value); err != nil {
			return nil, err
		}
		return e.Clip.Spawn(job), nil
	}
	w, tty, err := e.osc52Writer()
	if err != nil {
		return nil, err
	}
	if tty != nil {
		defer func() { _ = tty.Close() }()
		job.TTY = tty
	}
	if err := (clipboard.Terminal{W: w, Tmux: e.Clip.tmux()}).Write(value); err != nil {
		return nil, err
	}
	return e.Clip.Spawn(job), nil
}

func (e Env) osc52Writer() (io.Writer, *os.File, error) {
	if tty, err := e.Clip.OpenTTY(); err == nil {
		return tty, tty, nil
	}
	if e.Clip.IsTTY(e.Err) {
		return e.Err, nil, nil
	}
	return nil, nil, errNoOSC52Terminal
}
