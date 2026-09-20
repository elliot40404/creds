package cli

import (
	"fmt"
	"os"

	"github.com/elliot40404/creds/internal/app"
	"github.com/elliot40404/creds/internal/clipboard"
	"github.com/elliot40404/creds/internal/config"
	"github.com/elliot40404/creds/internal/render"
	"github.com/elliot40404/creds/internal/ui/tui"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

type picker struct {
	interactive func() bool
	open        func(tui.Options) error
	openSetup   func(tui.SetupOptions) error
}

func (e Env) systemPicker() picker {
	out, ok := e.Out.(*os.File)
	return picker{
		interactive: func() bool {
			return ok && term.IsTerminal(int(out.Fd())) && term.IsTerminal(int(os.Stdin.Fd()))
		},
		open: func(opts tui.Options) error {
			return tui.Run(opts, os.Stdin, out)
		},
		openSetup: func(opts tui.SetupOptions) error {
			return tui.RunSetup(opts, os.Stdin, out)
		},
	}
}

func (e Env) setupUI(u *app.Setup) error {
	return e.setupWith(u, e.systemPicker())
}

func (e Env) setupWith(u *app.Setup, pk picker) error {
	if !pk.interactive() {
		return e.setupFlow(u)
	}
	return pk.openSetup(tui.SetupOptions{Setup: u})
}

func pickCmd(env Env, root *cobra.Command) *cobra.Command {
	return pickCmdWith(env, root, env.systemPicker())
}

func pickCmdWith(env Env, root *cobra.Command, pk picker) *cobra.Command {
	run := func(cmd *cobra.Command, args []string) error {
		if !pk.interactive() {
			return root.Help()
		}
		return env.run(func(s *app.Service, _ []string) error {
			return env.pick(s, pk, layoutFlags(cmd, s.CurrentConfig().UI))
		})(cmd, args)
	}
	root.Args = cobra.NoArgs
	root.RunE = run
	pick := &cobra.Command{
		Use:   "pick",
		Short: "Browse, copy and manage entries",
		Args:  cobra.NoArgs,
		RunE:  run,
	}
	addLayoutFlags(root)
	addLayoutFlags(pick)
	return pick
}

const (
	flagInline     = "inline"
	flagFullscreen = "fullscreen"
	flagHeight     = "height"
	flagAltScreen  = "alt-screen"
)

func addLayoutFlags(c *cobra.Command) {
	fl := c.Flags()
	fl.Bool(flagInline, false, "draw under the prompt instead of filling the terminal")
	fl.Bool(flagFullscreen, false, "fill the terminal, overriding ui.mode")
	fl.Int(flagHeight, config.DefaultHeight, fmt.Sprintf("rows to use when inline, %d to %d", config.MinHeight, config.MaxHeight))
	fl.Bool(flagAltScreen, true, "swap to a clean screen while running, use --alt-screen=false to keep scrollback")
	c.MarkFlagsMutuallyExclusive(flagInline, flagFullscreen)
}

func layoutFlags(cmd *cobra.Command, ui config.UI) tui.Layout {
	l := tui.Layout{
		Inline:    ui.Mode == config.ModeInline,
		AltScreen: ui.AltScreen,
		Height:    ui.Height,
	}
	fl := cmd.Flags()
	if fl.Changed(flagInline) {
		l.Inline = true
	}
	if fl.Changed(flagFullscreen) {
		l.Inline = false
	}
	if fl.Changed(flagAltScreen) {
		l.AltScreen, _ = fl.GetBool(flagAltScreen)
	}
	if fl.Changed(flagHeight) {
		h, _ := fl.GetInt(flagHeight)
		l.Height = min(max(h, config.MinHeight), config.MaxHeight)
	}
	return l
}

func (e Env) pick(s *app.Service, pk picker, layout tui.Layout) error {
	if err := s.Unlock(); err != nil {
		return err
	}
	s.Prompter = tuiPrompter{}
	s.Spawn = nil
	return pk.open(tui.Options{
		Backend:   s,
		Config:    s,
		Copy:      e.tuiCopy(s),
		Hooks:     tui.DefaultHooks(),
		Unlock:    s.UnlockWith,
		VaultPath: s.Paths.Vault(),
		Layout:    layout,
	})
}

type tuiPrompter struct {
	noPrompt
}

func (tuiPrompter) Password(string) (string, error) {
	return "", tui.ErrNeedPassword
}

func (e Env) tuiCopy(s *app.Service) tui.Copier {
	return func(path, field, as string, sh render.Shell) (string, error) {
		label, warn, err := e.copyEntry(s, path, field, as, sh, clipboard.ModeAuto)
		if err != nil {
			return "", err
		}
		return clearNote(label, warn), nil
	}
}

func clearNote(label string, err error) string {
	if err == nil {
		return label
	}
	return fmt.Sprintf("%s, not cleared automatically: %v", label, err)
}
