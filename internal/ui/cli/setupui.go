package cli

import (
	"context"

	"github.com/elliot40404/creds/internal/app"
	"github.com/elliot40404/creds/internal/ui/tui"
)

type setupUI struct {
	app.Prompter
	e Env
}

func (s setupUI) SelectFrom(prompt string, options []string, _ int) (int, error) {
	return s.Select(prompt, options)
}

func (s setupUI) Checks(checks []app.Check) error {
	s.e.sayChecks(checks)
	return nil
}

func (s setupUI) Warn(msg string) {
	app.Warn(s.Prompter, msg)
}

func (s setupUI) Note(msg string) {
	s.e.say("%s", msg)
}

func (e Env) setupFlow(u *app.Setup) error {
	if err := tui.SetupFlow(context.Background(), u, setupUI{Prompter: u.Service.Prompter, e: e}); err != nil {
		return err
	}
	e.nextSteps(u)
	return nil
}
