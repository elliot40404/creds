package tui

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/elliot40404/creds/internal/app"
	"github.com/elliot40404/creds/internal/config"
	"github.com/elliot40404/creds/internal/fsutil"
	"github.com/elliot40404/creds/internal/gitsync"
	"github.com/elliot40404/creds/internal/testutil"
	"github.com/elliot40404/creds/internal/vault"
)

const (
	realWord   = "Tr0ub4dor&3x-zebra"
	realSecret = "charlie-secret-333"
)

type initPrompter struct {
	lockedPrompter
	code string
}

func (p *initPrompter) Password(string) (string, error) { return realWord, nil }

func (p *initPrompter) Input(string, string) (string, error) { return p.code, nil }

func (p *initPrompter) Show(_, text string) error {
	p.code = text
	return nil
}

type lockedPrompter struct{}

func (lockedPrompter) Password(string) (string, error) { return "", ErrNeedPassword }

func (lockedPrompter) Confirm(string) (bool, error) { return false, testutil.ErrNoAnswer }

func (lockedPrompter) Input(string, string) (string, error) { return "", testutil.ErrNoAnswer }

func (lockedPrompter) Select(string, []string) (int, error) { return 0, testutil.ErrNoAnswer }

func (lockedPrompter) Show(string, string) error { return nil }

type noWatch struct {
	Backend
}

var realFixture = sync.OnceValues(func() (string, error) {
	home := filepath.Join(fixtureDir, "real")
	s := realService(home, filepath.Join(fixtureDir, "user"), &initPrompter{})
	s.Keystore = testutil.NoKeystore
	if err := s.Init(); err != nil {
		return "", err
	}
	for _, e := range realEntries() {
		if _, err := s.Add(e); err != nil {
			return "", err
		}
	}
	return home, nil
})

func realEntries() []vault.Entry {
	return []vault.Entry{
		{Path: "web/mail", Type: vault.TypeLogin, Fields: []vault.Field{{Name: "password", Value: realSecret, Secret: true}}},
		{Path: "db/prod", Type: vault.TypeDatabase, Host: "h", Params: map[string]string{"engine": "postgres"}, Fields: []vault.Field{{Name: "password", Value: secretB, Secret: true}}},
	}
}

func realService(home, user string, p app.Prompter) *app.Service {
	c := config.Default()
	c.Sync.After = time.Millisecond
	return &app.Service{Paths: config.Paths{Home: home}, Config: c, Prompter: p, Now: time.Now, LogN: 10, UserHome: user, Keystore: testutil.Keystore}
}

func realCopy(t *testing.T) *app.Service {
	t.Helper()
	src, err := realFixture()
	if err != nil {
		t.Fatal(err)
	}
	home := t.TempDir()
	if err := os.CopyFS(home, os.DirFS(src)); err != nil {
		t.Fatal(err)
	}
	if err := fsutil.SecureTree(home); err != nil {
		t.Fatal(err)
	}
	s := realService(home, t.TempDir(), lockedPrompter{})
	if err := s.RemoteAdd(testutil.BareRemote(t, gitsync.Branch)); err != nil {
		t.Fatal(err)
	}
	return s
}

func realLoop(t *testing.T, autosync bool) (*loop, *app.Service) {
	t.Helper()
	s := realCopy(t)
	opts := Options{Backend: noWatch{Backend: s}, Hooks: DefaultHooks(), Unlock: s.UnlockWith, VaultPath: s.Paths.Vault()}
	if autosync {
		opts.Config = s
	}
	l := newLoop(t, New(opts))
	l.send(tea.WindowSizeMsg{Width: 120, Height: 30})
	l.exec(l.m.Init())
	l.settle()
	return l, s
}

func onTop[S Screen](m Model) bool {
	_, ok := m.top().(S)
	return ok
}

func idle(m Model) bool {
	return !m.op.busy() && !m.op.syncing
}

func sane(t *testing.T, l *loop, s *app.Service, pushed bool) {
	t.Helper()
	l.settle()
	if l.m.failed || !idle(l.m) {
		t.Fatalf("failed %v op %+v: %q", l.m.failed, l.m.op, view(l.m))
	}
	st, err := s.SyncStatus()
	if err != nil || st.LastResult == "error" || pushed && st.Ahead != 0 {
		t.Fatalf("sync status %+v err %v", st, err)
	}
}

func addEntry(l *loop, path string) {
	l.send(addMsg{})
	l.press(enter)
	l.press(text(path)...)
	l.press(ctrl('s'))
}

func TestRealSyncWhileOpening(t *testing.T) {
	t.Parallel()
	l, s := realLoop(t, false)
	l.send(syncMsg{})
	l.send(openMsg{path: "web/mail"})
	l.until("detail", onTop[detailScreen])
	sane(t, l, s, true)
	if d := l.m.top().(detailScreen); d.entry.Path != "web/mail" {
		t.Fatalf("opened %q", d.entry.Path)
	}
}

func TestRealSyncWhileSaving(t *testing.T) {
	t.Parallel()
	l, s := realLoop(t, false)
	l.send(syncMsg{})
	addEntry(l, "web/new")
	sane(t, l, s, false)
	if _, err := s.Get("web/new"); err != nil || !onTop[listScreen](l.m) {
		t.Fatalf("saved entry %v top %T", err, l.m.top())
	}
}

func TestRealAutoSyncWhileRevealing(t *testing.T) {
	t.Parallel()
	l, s := realLoop(t, true)
	addEntry(l, "web/new")
	l.until("save", func(m Model) bool { return strings.Contains(m.flash, "Saved") })
	l.send(openMsg{path: "web/mail"})
	l.until("detail", onTop[detailScreen])
	l.press(ch('r'))
	l.until("reveal", func(m Model) bool { return strings.Contains(view(m), realSecret) })
	sane(t, l, s, true)
	if !strings.Contains(view(l.m), realSecret) {
		t.Fatalf("secret hidden after sync: %q", view(l.m))
	}
}

func TestRealListWhileGetRecordsUsage(t *testing.T) {
	t.Parallel()
	l, s := realLoop(t, false)
	for _, path := range []string{"web/mail", "db/prod", "web/mail"} {
		l.send(openMsg{path: path})
		l.exec(l.m.loadItems(), l.m.loadItems())
		l.until("detail", onTop[detailScreen])
		l.press(esc)
		l.until("list", onTop[listScreen])
	}
	sane(t, l, s, false)
	if _, err := os.Stat(s.Paths.Usage()); err != nil {
		t.Fatalf("usage not recorded: %v", err)
	}
	if v := view(l.m); !strings.Contains(v, "web/mail") || !strings.Contains(v, "db/prod") {
		t.Fatalf("list %q", v)
	}
}
