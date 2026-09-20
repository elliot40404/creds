package tui

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/elliot40404/creds/internal/app"
	"github.com/elliot40404/creds/internal/config"
	"github.com/elliot40404/creds/internal/gitsync"
	"github.com/elliot40404/creds/internal/render"
	"github.com/elliot40404/creds/internal/testutil"
)

const testPassword = "correct horse battery staple"

type scripted struct {
	passwords []string
	inputs    []string
	picks     []string
	confirms  []bool
	asked     []string
	shown     []string
	notes     []string
	checks    []app.Check
	curs      []int
	offered   [][]string
	t         *testing.T
}

func (s *scripted) Password(string) (string, error) {
	return take(s.t, &s.passwords, "password")
}

func (s *scripted) Input(prompt, def string) (string, error) {
	if strings.Contains(strings.ToLower(prompt), "recovery") && len(s.shown) > 0 {
		return s.shown[len(s.shown)-1], nil
	}
	v, err := take(s.t, &s.inputs, "input")
	if v == "" {
		return def, err
	}
	return v, err
}

func (s *scripted) Select(_ string, options []string) (int, error) {
	s.offered = append(s.offered, options)
	want, err := take(s.t, &s.picks, "select")
	if err != nil {
		return 0, err
	}
	for i, o := range options {
		if o == want {
			return i, nil
		}
	}
	s.t.Fatalf("pick %q not in %v", want, options)
	return 0, nil
}

func (s *scripted) SelectFrom(prompt string, options []string, cur int) (int, error) {
	s.curs = append(s.curs, cur)
	return s.Select(prompt, options)
}

func (s *scripted) Confirm(prompt string) (bool, error) {
	s.asked = append(s.asked, prompt)
	if len(s.confirms) == 0 {
		s.t.Fatal("unexpected confirm")
	}
	v := s.confirms[0]
	s.confirms = s.confirms[1:]
	return v, nil
}

func (s *scripted) Show(_, text string) error {
	s.shown = append(s.shown, text)
	return nil
}

func (s *scripted) Checks(checks []app.Check) error {
	s.checks = checks
	return nil
}

func (s *scripted) Note(msg string) {
	s.notes = append(s.notes, msg)
}

func (s *scripted) noteText() string {
	return strings.Join(s.notes, "\n")
}

func take(t *testing.T, queue *[]string, what string) (string, error) {
	t.Helper()
	v, err := testutil.Pop(queue)
	if err != nil {
		t.Fatalf("no %s left", what)
	}
	return v, nil
}

type fakeHost struct {
	url     string
	repos   []string
	created []string
	err     error
}

func (h *fakeHost) Available(context.Context) error { return h.err }

func (h *fakeHost) Create(_ context.Context, name string) (string, error) {
	h.created = append(h.created, name)
	return h.url, nil
}

func (h *fakeHost) Repos(context.Context) ([]string, error) { return h.repos, h.err }

func newSetupUnit(t *testing.T, ui app.Prompter) *app.Setup {
	t.Helper()
	home := filepath.Join(t.TempDir(), "home")
	s := &app.Service{Paths: config.Paths{Home: home}, Config: config.Default(), Prompter: ui, LogN: 10}
	u := s.Setup()
	u.OS = "linux"
	u.Look = func(file string) (string, error) {
		if file == "git" || file == "wl-copy" {
			return "/usr/bin/" + file, nil
		}
		return "", errors.New("not found")
	}
	return u
}

func TestFlowNewVaultNoRemote(t *testing.T) {
	ui := &scripted{t: t, picks: []string{modeNew, remoteSkip}, passwords: []string{testPassword, testPassword}, confirms: []bool{false}}
	u := newSetupUnit(t, ui)
	if err := SetupFlow(context.Background(), u, ui); err != nil {
		t.Fatal(err)
	}
	if len(ui.shown) != 1 || ui.shown[0] == "" {
		t.Fatalf("recovery code shown %v", ui.shown)
	}
	if !strings.Contains(ui.noteText(), "vault created") {
		t.Fatalf("notes %q", ui.noteText())
	}
	if ok, err := u.Service.HasVault(); err != nil || !ok {
		t.Fatalf("vault missing: %v %v", ok, err)
	}
	if len(ui.checks) != 5 {
		t.Fatalf("checks %v", ui.checks)
	}
}

func TestFlowPreflightHomeBroken(t *testing.T) {
	ui := &scripted{t: t}
	u := newSetupUnit(t, ui)
	u.Service.Paths = config.Paths{Home: filepath.Join(t.TempDir(), "file", "home")}
	if err := os.WriteFile(filepath.Dir(u.Service.Paths.Home), []byte("x"), 0o600); err != nil {
		t.Skipf("cannot block home: %v", err)
	}
	err := SetupFlow(context.Background(), u, ui)
	if !errors.Is(err, ErrHomeBroken) {
		t.Fatalf("err %v", err)
	}
}

func TestFlowSettingsRetriesBadDuration(t *testing.T) {
	ui := &scripted{t: t, confirms: []bool{true}, inputs: []string{"nope", "20m", "9h"}, picks: []string{string(render.Pwsh)}}
	u := newSetupUnit(t, ui)
	if err := setupSettings(u, ui); err != nil {
		t.Fatal(err)
	}
	set := u.Settings()
	if set.Idle.String() != "20m0s" || set.Hard.String() != "9h0m0s" || set.Shell != render.Pwsh {
		t.Fatalf("settings %+v", set)
	}
}

func TestFlowRemoteGHCreatesRepo(t *testing.T) {
	host := &fakeHost{url: "git@github.com:me/creds-vault.git"}
	ui := &scripted{t: t, picks: []string{remoteGH}, inputs: []string{""}, confirms: []bool{true, false}}
	u := newSetupUnit(t, ui)
	u.Host = host
	u.Probe = func(context.Context, string) (app.RemoteState, error) { return app.RemoteEmpty, nil }
	if err := setupRemote(t.Context(), u, ui); err != nil {
		t.Fatal(err)
	}
	if len(host.created) != 1 || host.created[0] != "creds-vault" {
		t.Fatalf("created %v", host.created)
	}
	if !strings.Contains(ui.noteText(), host.url) {
		t.Fatalf("notes %q", ui.noteText())
	}
}

func TestFlowRemoteSkipsGHWhenMissing(t *testing.T) {
	ui := &scripted{t: t, picks: []string{remoteSkip}}
	u := newSetupUnit(t, ui)
	u.Host = &fakeHost{err: errors.New("gh not found")}
	if err := setupRemote(t.Context(), u, ui); err != nil {
		t.Fatal(err)
	}
	if len(ui.notes) != 0 {
		t.Fatalf("notes %v", ui.notes)
	}
}

func TestFlowJoinPicksGHRepo(t *testing.T) {
	host := &fakeHost{repos: []string{"git@github.com:me/vault.git", "git@github.com:me/other.git"}}
	pick := "git@github.com:me/other.git"
	ui := &scripted{t: t, picks: []string{pick, pick, pick}}
	u := newSetupUnit(t, ui)
	u.Host = host
	u.Probe = func(context.Context, string) (app.RemoteState, error) { return app.RemoteEmpty, nil }
	err := setupJoin(context.Background(), u, ui)
	if !errors.Is(err, ErrRemoteNoVault) || !strings.Contains(err.Error(), "other.git") {
		t.Fatalf("err %v", err)
	}
}

func TestFlowJoinPastesURL(t *testing.T) {
	host := &fakeHost{repos: []string{"git@github.com:me/vault.git"}}
	ui := &scripted{t: t, picks: []string{pasteURL, pasteURL, pasteURL}, inputs: slices.Repeat([]string{"git@example.com:me/v.git"}, app.MaxTries)}
	u := newSetupUnit(t, ui)
	u.Host = host
	u.Probe = func(_ context.Context, url string) (app.RemoteState, error) {
		if url != "git@example.com:me/v.git" {
			t.Fatalf("probed %q", url)
		}
		return app.RemoteOther, nil
	}
	if err := setupJoin(context.Background(), u, ui); !errors.Is(err, ErrRemoteNoVault) {
		t.Fatalf("err %v", err)
	}
}

func TestFlowJoinWithoutHostAsksURL(t *testing.T) {
	ui := &scripted{t: t, inputs: slices.Repeat([]string{"git@example.com:me/v.git"}, app.MaxTries)}
	u := newSetupUnit(t, ui)
	probed := ""
	u.Probe = func(_ context.Context, url string) (app.RemoteState, error) {
		probed = url
		return app.RemoteEmpty, nil
	}
	if err := setupJoin(context.Background(), u, ui); !errors.Is(err, ErrRemoteNoVault) {
		t.Fatalf("err %v", err)
	}
	if probed != "git@example.com:me/v.git" {
		t.Fatalf("probed %q", probed)
	}
}

func TestEntryCount(t *testing.T) {
	if entryCount(1) != "1 entry" || entryCount(0) != "0 entries" {
		t.Fatal("entryCount")
	}
}

func TestFlowShellSelectPreselectsCurrent(t *testing.T) {
	ui := &scripted{t: t, confirms: []bool{true}, inputs: []string{"15m", "8h"}, picks: []string{string(render.Bash)}}
	u := newSetupUnit(t, ui)
	set := u.Settings()
	set.Shell = render.Pwsh
	if err := u.SaveSettings(set); err != nil {
		t.Fatal(err)
	}
	if err := setupSettings(u, ui); err != nil {
		t.Fatal(err)
	}
	if len(ui.curs) != 1 || ui.curs[0] != 1 {
		t.Fatalf("preselected %v, want pwsh", ui.curs)
	}
}

func TestFlowJoinPickerOffersPasteFirst(t *testing.T) {
	ui := &scripted{t: t, picks: []string{pasteURL}, inputs: []string{"git@example.com:me/v.git"}}
	u := newSetupUnit(t, ui)
	u.Host = &fakeHost{repos: []string{"git@github.com:me/vault.git"}}
	url, err := joinURL(t.Context(), u, ui)
	if err != nil {
		t.Fatal(err)
	}
	if url != "git@example.com:me/v.git" {
		t.Fatalf("url %q", url)
	}
	if len(ui.offered) == 0 || ui.offered[0][0] != pasteURL {
		t.Fatalf("options %v", ui.offered)
	}
}

func TestFlowRemoteKeepsVaultWhenRemoteHasVault(t *testing.T) {
	ui := &scripted{t: t, picks: []string{modeNew, remoteURL, remoteSkip}, inputs: []string{"git@example.com:me/v.git"}, passwords: []string{testPassword, testPassword}, confirms: []bool{false}}
	u := newSetupUnit(t, ui)
	u.Probe = func(context.Context, string) (app.RemoteState, error) { return app.RemoteVault, nil }
	if err := SetupFlow(context.Background(), u, ui); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(ui.noteText(), keptVault) {
		t.Fatalf("notes %q", ui.noteText())
	}
	if ok, err := u.Service.HasVault(); err != nil || !ok {
		t.Fatalf("vault removed: %v %v", ok, err)
	}
}

func TestFlowRemoteAsksAgainWhenRepoMissing(t *testing.T) {
	ui := &scripted{t: t, picks: []string{modeNew, remoteURL, remoteSkip}, inputs: []string{"git@example.com:me/gone.git"}, passwords: []string{testPassword, testPassword}, confirms: []bool{false}}
	u := newSetupUnit(t, ui)
	u.Probe = func(context.Context, string) (app.RemoteState, error) {
		return app.RemoteUnknown, gitsync.ErrRepoNotFound
	}
	if err := SetupFlow(context.Background(), u, ui); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(ui.noteText(), keptVault) {
		t.Fatalf("notes %q", ui.noteText())
	}
}

func TestFlowWithVaultOffersManage(t *testing.T) {
	ui := &scripted{t: t, picks: []string{modeNew, remoteSkip}, passwords: []string{testPassword, testPassword}, confirms: []bool{false}}
	u := newSetupUnit(t, ui)
	if err := SetupFlow(context.Background(), u, ui); err != nil {
		t.Fatal(err)
	}
	again := &scripted{t: t, picks: []string{manageSteps}}
	if err := SetupFlow(context.Background(), u, again); err != nil {
		t.Fatal(err)
	}
	if len(again.offered) != 1 || !slices.Contains(again.offered[0], manageAttach) {
		t.Fatalf("options %v", again.offered)
	}
}

func TestFlowManageChangesSettings(t *testing.T) {
	ui := &scripted{t: t, picks: []string{modeNew, remoteSkip}, passwords: []string{testPassword, testPassword}, confirms: []bool{false}}
	u := newSetupUnit(t, ui)
	if err := SetupFlow(context.Background(), u, ui); err != nil {
		t.Fatal(err)
	}
	again := &scripted{t: t, picks: []string{manageSettings, string(render.Pwsh)}, confirms: []bool{true}, inputs: []string{"20m", "9h"}}
	if err := SetupFlow(context.Background(), u, again); err != nil {
		t.Fatal(err)
	}
	if set := u.Settings(); set.Idle.String() != "20m0s" || set.Shell != render.Pwsh {
		t.Fatalf("settings %+v", set)
	}
}

func TestFlowHidesURLToken(t *testing.T) {
	ui := &scripted{t: t, picks: []string{remoteURL}, inputs: []string{"https://user:SECRETTOKEN@example.invalid/x.git"}, confirms: []bool{false}}
	u := newSetupUnit(t, ui)
	u.Host = &fakeHost{err: errors.New("gh not found")}
	u.Probe = func(context.Context, string) (app.RemoteState, error) { return app.RemoteEmpty, nil }
	if err := setupRemote(t.Context(), u, ui); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(ui.asked, []string{"Push the vault to https://example.invalid/x.git?"}) {
		t.Fatalf("asked %v", ui.asked)
	}
	join := &scripted{t: t, inputs: slices.Repeat([]string{"https://user:SECRETTOKEN@example.invalid/x.git"}, app.MaxTries)}
	u = newSetupUnit(t, join)
	u.Probe = func(context.Context, string) (app.RemoteState, error) { return app.RemoteEmpty, nil }
	err := setupJoin(context.Background(), u, join)
	if !errors.Is(err, ErrRemoteNoVault) || strings.Contains(err.Error(), "SECRETTOKEN") {
		t.Fatalf("err %v", err)
	}
}

func TestFlowManageHidesURLToken(t *testing.T) {
	ui := &scripted{t: t, picks: []string{modeNew, remoteSkip}, passwords: []string{testPassword, testPassword}, confirms: []bool{false}}
	u := newSetupUnit(t, ui)
	if err := SetupFlow(context.Background(), u, ui); err != nil {
		t.Fatal(err)
	}
	if err := u.Service.SetRemote("https://user:SECRETTOKEN@example.invalid/x.git"); err != nil {
		t.Fatal(err)
	}
	again := &scripted{t: t, picks: []string{manageSteps}}
	if err := SetupFlow(context.Background(), u, again); err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(again.notes, "remote: https://example.invalid/x.git") || strings.Contains(again.noteText(), "SECRETTOKEN") {
		t.Fatalf("notes %q", again.noteText())
	}
}
