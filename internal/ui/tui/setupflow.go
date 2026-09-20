package tui

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/elliot40404/creds/internal/app"
	"github.com/elliot40404/creds/internal/config"
	"github.com/elliot40404/creds/internal/render"
	"github.com/elliot40404/creds/internal/safetext"
)

var (
	ErrRemoteNoVault = errors.New("that remote has no creds vault")
	ErrHomeBroken    = errors.New("creds home is not usable")
)

const (
	modeNew    = "Create a new vault"
	modeJoin   = "Join a vault that already exists"
	remoteGH   = "Create a private GitHub repo with gh"
	remoteURL  = "Paste a git url"
	remoteSkip = "Skip sync for now"
	pasteURL   = "Paste a git url instead"

	manageAttach   = "Attach a git remote"
	manageChange   = "Change the git remote"
	manageSettings = "Change settings"
	manageSteps    = "Show next steps"

	keptVault = "vault kept, pick another remote or skip sync"
)

type SetupAsker interface {
	app.Prompter
	SelectFrom(prompt string, options []string, cur int) (int, error)
	Checks(checks []app.Check) error
	Note(msg string)
}

func SetupFlow(ctx context.Context, u *app.Setup, ui SetupAsker) error {
	if err := setupFlow(ctx, u, ui); err != nil {
		return err
	}
	return u.Service.Unlock()
}

func setupFlow(ctx context.Context, u *app.Setup, ui SetupAsker) error {
	if err := setupPreflight(u, ui); err != nil {
		return err
	}
	has, err := u.Service.HasVault()
	if err != nil {
		return err
	}
	if has {
		return setupManage(ctx, u, ui)
	}
	if err := u.Service.CheckFresh(); err != nil {
		return err
	}
	i, err := ui.Select("What do you want to do", []string{modeNew, modeJoin})
	if err != nil {
		return err
	}
	if app.SetupMode(i) == app.SetupJoin {
		return setupJoin(ctx, u, ui)
	}
	return setupNew(ctx, u, ui)
}

func setupManage(ctx context.Context, u *app.Setup, ui SetupAsker) error {
	remote := u.Service.RemoteURL()
	label := manageAttach
	if remote != "" {
		label = manageChange
		ui.Note("remote: " + safetext.Remote(remote))
	}
	i, err := ui.Select("This home already has a vault, what do you want to do", []string{label, manageSettings, manageSteps})
	if err != nil {
		return err
	}
	switch i {
	case 0:
		return setupRemote(ctx, u, ui)
	case 1:
		return setupSettings(u, ui)
	}
	return nil
}

func setupPreflight(u *app.Setup, ui SetupAsker) error {
	checks := u.Preflight()
	if err := ui.Checks(checks); err != nil {
		return err
	}
	for _, c := range checks {
		if !c.OK && c.Name == "home" {
			return fmt.Errorf("%w: %s", ErrHomeBroken, c.Message)
		}
	}
	return nil
}

func setupNew(ctx context.Context, u *app.Setup, ui SetupAsker) error {
	if err := u.Service.Init(); err != nil {
		return err
	}
	ui.Note("vault created")
	if err := setupRemote(ctx, u, ui); err != nil {
		return err
	}
	return setupSettings(u, ui)
}

func setupRemote(ctx context.Context, u *app.Setup, ui SetupAsker) error {
	for try := 1; ; try++ {
		err := app.SetupRemoteErr(remoteStep(ctx, u, ui))
		if err == nil || try == app.MaxTries || !app.RemoteRetryable(err) {
			return err
		}
		app.Warn(ui, err.Error())
		ui.Note(keptVault)
	}
}

func remoteStep(ctx context.Context, u *app.Setup, ui SetupAsker) error {
	labels := []string{remoteURL, remoteSkip}
	if u.HostAvailable(ctx) == nil {
		labels = append([]string{remoteGH}, labels...)
	}
	i, err := ui.Select("Where should the vault sync to", labels)
	if err != nil {
		return err
	}
	switch labels[i] {
	case remoteGH:
		return setupGHRemote(ctx, u, ui)
	case remoteURL:
		url, err := ui.Input("Git url of an empty repository", "")
		if err != nil {
			return err
		}
		return setupPush(ctx, u, ui, url)
	}
	return nil
}

func setupGHRemote(ctx context.Context, u *app.Setup, ui SetupAsker) error {
	name, err := ui.Input("Repository name", "creds-vault")
	if err != nil {
		return err
	}
	ok, err := ui.Confirm("Create private GitHub repo " + name + "?")
	if err != nil || !ok {
		return err
	}
	url, err := u.CreateRepo(ctx, name)
	if err != nil {
		return err
	}
	ui.Note("created " + url)
	return setupPush(ctx, u, ui, url)
}

func setupPush(ctx context.Context, u *app.Setup, ui SetupAsker, url string) error {
	if err := u.CheckRemoteEmpty(ctx, url); err != nil {
		return err
	}
	ok, err := ui.Confirm("Push the vault to " + safetext.Remote(url) + "?")
	if err != nil || !ok {
		return err
	}
	if err := u.ReplaceRemote(url); err != nil {
		return err
	}
	ui.Note("vault pushed to " + safetext.Remote(url))
	return nil
}

func setupSettings(u *app.Setup, ui SetupAsker) error {
	ok, err := ui.Confirm("Change idle timeout, hard timeout or render shell?")
	if err != nil || !ok {
		return err
	}
	set := u.Settings()
	if set.Idle, err = askDuration(ui, "Idle timeout", set.Idle); err != nil {
		return err
	}
	if set.Hard, err = askDuration(ui, "Hard timeout", set.Hard); err != nil {
		return err
	}
	shells := render.Shells()
	i, err := ui.SelectFrom("Shell for creds env and creds run", shells, slices.Index(shells, string(set.Shell)))
	if err != nil {
		return err
	}
	set.Shell = render.Shell(shells[i])
	return u.SaveSettings(set)
}

func askDuration(ui SetupAsker, prompt string, def time.Duration) (time.Duration, error) {
	return app.Retry(ui, func() (time.Duration, error) {
		ans, err := ui.Input(prompt, def.String())
		if err != nil {
			return 0, err
		}
		d, err := time.ParseDuration(ans)
		if err != nil {
			return 0, fmt.Errorf("%w: %q", config.ErrBadDuration, ans)
		}
		return d, nil
	}, config.ErrBadDuration)
}

func setupJoin(ctx context.Context, u *app.Setup, ui SetupAsker) error {
	url, err := app.Retry(ui, func() (string, error) {
		return joinTarget(ctx, u, ui)
	}, ErrRemoteNoVault)
	if err != nil {
		return err
	}
	if err := u.Service.Join(url); err != nil {
		return err
	}
	items, err := u.Service.List()
	if err != nil {
		return err
	}
	ui.Note(fmt.Sprintf("joined %s, %s", safetext.Remote(url), entryCount(len(items))))
	return nil
}

func joinTarget(ctx context.Context, u *app.Setup, ui SetupAsker) (string, error) {
	url, err := joinURL(ctx, u, ui)
	if err != nil {
		return "", err
	}
	state, err := u.ProbeRemote(ctx, url)
	if err != nil {
		return "", err
	}
	if state != app.RemoteVault {
		return "", fmt.Errorf("%w: %s", ErrRemoteNoVault, safetext.Remote(url))
	}
	return url, nil
}

func joinURL(ctx context.Context, u *app.Setup, ui SetupAsker) (string, error) {
	repos := ownRepos(ctx, u)
	if len(repos) == 0 {
		return ui.Input("Git url of the vault", "")
	}
	i, err := ui.Select("Pick the repository that holds the vault", append([]string{pasteURL}, repos...))
	if err != nil {
		return "", err
	}
	if i == 0 {
		return ui.Input("Git url of the vault", "")
	}
	return repos[i-1], nil
}

func ownRepos(ctx context.Context, u *app.Setup) []string {
	if u.HostAvailable(ctx) != nil {
		return nil
	}
	repos, err := u.HostRepos(ctx)
	if err != nil {
		return nil
	}
	return repos
}

func entryCount(n int) string {
	if n == 1 {
		return "1 entry"
	}
	return fmt.Sprintf("%d entries", n)
}
