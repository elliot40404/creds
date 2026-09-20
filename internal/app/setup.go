package app

import (
	"context"
	"errors"
	"slices"
	"time"

	"github.com/elliot40404/creds/internal/render"
)

type SetupMode int

const (
	SetupNew SetupMode = iota
	SetupJoin
)

type RemoteState int

const (
	RemoteUnknown RemoteState = iota
	RemoteEmpty
	RemoteVault
	RemoteOther
)

var (
	ErrRemoteHasVault = errors.New("this remote already holds a creds vault, pick join instead of create")
	ErrRemoteNotEmpty = errors.New("remote is not empty and is not a creds vault")
	errRemoteMissing  = errors.New("remote repository not found, check the url or create the repository first")
	ErrNoHost         = errors.New("no repository host configured")
	ErrNoLookup       = errors.New("no program lookup configured")
)

type Check struct {
	Name    string
	OK      bool
	Message string
	Fix     string
}

type Settings struct {
	Idle  time.Duration
	Hard  time.Duration
	Shell render.Shell
}

type Hoster interface {
	Available(ctx context.Context) error
	Create(ctx context.Context, name string) (string, error)
	Repos(ctx context.Context) ([]string, error)
}

type Setup struct {
	Service *Service
	Host    Hoster
	Look    func(file string) (string, error)
	OS      string
	Probe   func(ctx context.Context, url string) (RemoteState, error)
}

func (s *Service) Setup() *Setup {
	return &Setup{Service: s}
}

func (s *Service) HasVault() (bool, error) {
	return s.hasVault()
}

func (u *Setup) Settings() Settings {
	c := u.Service.CurrentConfig()
	return Settings{Idle: c.Session.Idle, Hard: c.Session.Hard, Shell: c.Render.Shell}
}

func (u *Setup) SaveSettings(set Settings) error {
	c := u.Service.CurrentConfig()
	c.Session.Idle, c.Session.Hard, c.Render.Shell = set.Idle, set.Hard, set.Shell
	return u.Service.saveConfig(c)
}

func (u *Setup) NextSteps() []string {
	return []string{
		"creds add -i to add your first entry",
		"creds to browse, copy and manage entries",
		"creds sync to push and pull the vault",
		"creds help agents for the scripting guide",
		completionStep(u.Service.RenderShell()),
	}
}

func ShellChoices(cur render.Shell) []string {
	all := []string{string(render.Bash), string(render.Pwsh)}
	if i := slices.Index(all, string(cur)); i > 0 {
		all[0], all[i] = all[i], all[0]
	}
	return all
}

func completionStep(shell render.Shell) string {
	if shell == render.Pwsh {
		return "creds completion powershell | Out-String | Invoke-Expression, add it to $PROFILE to keep it"
	}
	return "source <(creds completion bash), add it to your shell rc to keep it"
}

func (u *Setup) hoster() (Hoster, error) {
	if u.Host == nil {
		return nil, ErrNoHost
	}
	return u.Host, nil
}

func (u *Setup) HostAvailable(ctx context.Context) error {
	h, err := u.hoster()
	if err != nil {
		return err
	}
	return h.Available(ctx)
}

func (u *Setup) HostRepos(ctx context.Context) ([]string, error) {
	h, err := u.hoster()
	if err != nil {
		return nil, err
	}
	return h.Repos(ctx)
}

func (u *Setup) CreateRepo(ctx context.Context, name string) (string, error) {
	if err := u.HostAvailable(ctx); err != nil {
		return "", err
	}
	return u.Host.Create(ctx, name)
}
