package cli

import (
	"encoding/json/v2"
	"os"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/elliot40404/creds/internal/config"
	"github.com/elliot40404/creds/internal/gitsync"
)

func TestQuietEnv(t *testing.T) {
	t.Parallel()
	got := quietEnv([]string{"A=1"}, "")
	for _, want := range []string{"A=1", "GIT_TERMINAL_PROMPT=0", "GIT_SSH_COMMAND=ssh -o BatchMode=yes"} {
		if !slices.Contains(got, want) {
			t.Fatalf("missing %q in %v", want, got)
		}
	}
	got = quietEnv(nil, "ssh -i key")
	if !slices.Contains(got, "GIT_SSH_COMMAND=ssh -i key -o BatchMode=yes") {
		t.Fatalf("env = %v", got)
	}
}

func (h *harness) spawnRun(n *int, f *fake, args ...string) {
	h.t.Helper()
	env, _, errb := h.env(f)
	env.Spawn = func() error {
		*n++
		return nil
	}
	if code := Run(env, args); code != 0 {
		h.t.Fatalf("%v: %d %q", args, code, errb.String())
	}
}

func TestCommandsSpawnSync(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.init()
	h.ok(&fake{}, "remote", "add", bareRemote(t))
	n := 0
	h.spawnRun(&n, &fake{inputs: []string{"make deploy", ""}}, "add", "--type", "command", "ops/deploy")
	if n != 1 {
		t.Fatalf("add spawns = %d", n)
	}
	h.spawnRun(&n, &fake{}, "list")
	if n != 1 {
		t.Fatalf("a read right after a spawn started another one: spawns = %d", n)
	}
	h.ok(&fake{}, "sync")
	h.spawnRun(&n, &fake{}, "get", "ops/deploy")
	h.spawnRun(&n, &fake{}, "status")
	if n != 1 {
		t.Fatalf("fresh read spawns = %d", n)
	}
}

func TestSyncFailureBanner(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.init()
	remote := bareRemote(t)
	h.ok(&fake{}, "remote", "add", remote)
	if err := os.Rename(remote, remote+".gone"); err != nil {
		t.Fatal(err)
	}
	h.ok(&fake{}, "sync", "--quiet")
	r := h.ok(&fake{}, "list")
	want := "remote repository not found, check the url in creds status"
	if !strings.HasPrefix(r.err, "last sync failed: "+want) || strings.Contains(r.err, "git fetch") {
		t.Fatalf("banner = %q", r.err)
	}
	if r = h.ok(&fake{}, "status"); strings.Contains(r.err, "last sync failed") || !strings.Contains(r.out, "last error: "+want) {
		t.Fatalf("status = %q %q", r.out, r.err)
	}
	var st struct {
		LastError    string `json:"last_error"`
		LastErrorRaw string `json:"last_error_raw"`
	}
	if r = h.ok(&fake{}, "status", "--json"); json.Unmarshal([]byte(r.out), &st) != nil || st.LastError != "remote repository not found" || !strings.HasPrefix(st.LastErrorRaw, "git fetch") {
		t.Fatalf("status json = %q", r.out)
	}
	if err := os.Rename(remote+".gone", remote); err != nil {
		t.Fatal(err)
	}
	if r = h.ok(&fake{}, "sync"); strings.Contains(r.err, "last sync failed") {
		t.Fatalf("sync = %q", r.err)
	}
	if r = h.ok(&fake{}, "list"); r.err != "" {
		t.Fatalf("banner not cleared: %q", r.err)
	}
}

func TestInteractiveSpawnsAtMostOnce(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.init()
	h.ok(&fake{}, "remote", "add", bareRemote(t))
	h.ok(&fake{inputs: []string{"make deploy", ""}}, "add", "--type", "command", "ops/deploy")
	n := 0
	h.spawnRun(&n, &fake{inputs: []string{""}, picks: []string{"ops/deploy", "get", actionEntry}, confirms: []bool{false}}, "list", "-i")
	if n != 1 {
		t.Fatalf("list -i spawns = %d", n)
	}
}

func TestSyncPrintsWaiting(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.init()
	h.ok(&fake{}, "remote", "add", bareRemote(t))
	env, _, _ := h.env(&fake{})
	w := &signalWriter{want: "waiting for another sync...\n", hit: make(chan struct{})}
	env.Err = w
	paths := config.Paths{Home: env.Home}
	lock, err := gitsync.AcquireLock(paths.SyncLock(), h.now)
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan int, 1)
	go func() { done <- Run(env, []string{"sync"}) }()
	select {
	case <-w.hit:
	case code := <-done:
		t.Fatalf("sync returned %d without waiting: %q", code, w.String())
	}
	if err := lock.Release(); err != nil {
		t.Fatal(err)
	}
	if code := <-done; code != 0 {
		t.Fatalf("sync: %d %q", code, w.String())
	}
}

type signalWriter struct {
	mu   sync.Mutex
	buf  strings.Builder
	want string
	hit  chan struct{}
	seen bool
}

func (w *signalWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	n, err := w.buf.Write(p)
	if !w.seen && strings.Contains(w.buf.String(), w.want) {
		w.seen = true
		close(w.hit)
	}
	return n, err
}

func (w *signalWriter) String() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.buf.String()
}
