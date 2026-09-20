package cli

import (
	"context"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/elliot40404/creds/internal/gitsync"
	"github.com/elliot40404/creds/internal/testutil"
)

func bareRemote(t *testing.T) string {
	t.Helper()
	return testutil.BareRemote(t, gitsync.Branch)
}

func remoteLog(t *testing.T, dir string) string {
	t.Helper()
	out, err := gitsync.NewGit(dir).Run(context.Background(), "log", "--format=%B", gitsync.Branch)
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}

func (h *harness) addCommand(path, cmd string) {
	h.t.Helper()
	h.ok(&fake{inputs: []string{cmd, ""}}, "add", "--type", "command", path)
}

func TestRemoteSyncPushes(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.init()
	h.addCommand("team/secret-name", "make deploy")
	remote := bareRemote(t)
	h.fail(&fake{}, "sync")
	h.ok(&fake{}, "remote", "add", remote)
	h.fail(&fake{}, "remote", "add", remote)
	r := h.ok(&fake{}, "status")
	if !strings.Contains(r.out, "remote: "+remote) || !strings.Contains(r.out, "ahead: 2") || !strings.Contains(r.out, "last sync: never") {
		t.Fatalf("status = %q", r.out)
	}
	r = h.ok(&fake{}, "sync")
	if !strings.Contains(r.err, "sync: pushed") {
		t.Fatalf("sync = %q", r.err)
	}
	log := remoteLog(t, remote)
	if strings.Count(log, "update vault") != 2 || strings.Contains(log, "secret") || strings.Contains(log, "team") {
		t.Fatalf("remote log = %q", log)
	}
	r = h.ok(&fake{}, "status")
	if !strings.Contains(r.out, "ahead: 0") || !strings.Contains(r.out, "(pushed)") {
		t.Fatalf("status = %q", r.out)
	}
	h.ok(&fake{}, "remote", "remove")
	r = h.ok(&fake{}, "status")
	if !strings.Contains(r.out, "remote: none") {
		t.Fatalf("status = %q", r.out)
	}
}

func TestResolveNeedsOneSide(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.init()
	for _, args := range [][]string{{"resolve", "x"}, {"resolve", "x", "--mine", "--theirs"}} {
		r := h.fail(&fake{}, args...)
		if !strings.Contains(r.err, errSide.Error()) {
			t.Fatalf("%v: %q", args, r.err)
		}
	}
	h.ok(&fake{}, "remote", "add", bareRemote(t))
	r := h.fail(&fake{}, "resolve", "x", "--mine", "--yes")
	if !strings.Contains(r.err, "no pending conflict") {
		t.Fatalf("err = %q", r.err)
	}
}

func (h *harness) editDeploy(cmd string) {
	h.t.Helper()
	h.ok(&fake{}, "edit", "ops/deploy", "--field", "command="+cmd)
}

func joined(t *testing.T) (a, b *harness) {
	t.Helper()
	remote := bareRemote(t)
	a = newHarness(t)
	a.init()
	a.addCommand("ops/deploy", "make deploy")
	a.ok(&fake{}, "remote", "add", remote)
	a.ok(&fake{}, "sync")
	b = newHarness(t)
	b.ok(&fake{passwords: []string{mainWord}}, "join", "--yes", remote)
	return a, b
}

func TestJoinSeesEntries(t *testing.T) {
	t.Parallel()
	a, b := joined(t)
	r := b.ok(&fake{}, "get", "ops/deploy")
	if !strings.Contains(r.out, "make deploy") {
		t.Fatalf("get = %q", r.out)
	}
	b.addCommand("ops/build", "make build")
	b.ok(&fake{}, "sync")
	a.ok(&fake{}, "sync")
	a.ok(&fake{}, "get", "ops/build")
	b.fail(&fake{}, "join", "--yes", "whatever")
}

func TestJoinFailureCleansUp(t *testing.T) {
	t.Parallel()
	remote := bareRemote(t)
	a := newHarness(t)
	a.init()
	a.ok(&fake{}, "remote", "add", remote)
	a.ok(&fake{}, "sync")
	b := newHarness(t)
	r := b.fail(&fake{passwords: []string{nextWord}}, "join", "--yes", remote)
	if strings.Contains(r.err, nextWord) {
		t.Fatal("error leaks password")
	}
	b.fail(&fake{}, "join", "--yes", filepath.Join(t.TempDir(), "missing.git"))
	b.ok(&fake{passwords: []string{mainWord}}, "join", "--yes", remote)
}

func TestConflictReportedThenResolved(t *testing.T) {
	t.Parallel()
	a, b := joined(t)
	b.editDeploy("make theirs")
	b.ok(&fake{}, "sync")
	a.editDeploy("make mine")
	r := a.run(&fake{}, "sync")
	if r.code != 5 || !strings.Contains(r.err, "conflict: ops/deploy") || !strings.Contains(r.err, "resolve") {
		t.Fatalf("sync = %q", r.err)
	}
	if out := a.ok(&fake{}, "get", "ops/deploy").out; !strings.Contains(out, "make mine") {
		t.Fatalf("local changed: %q", out)
	}
	if out := a.ok(&fake{}, "status").out; !strings.Contains(out, "conflict: ops/deploy") {
		t.Fatalf("status = %q", out)
	}
	a.ok(&fake{}, "resolve", "ops/deploy", "--mine", "--yes")
	if out := a.ok(&fake{}, "status").out; strings.Contains(out, "conflict") || !strings.Contains(out, "ahead: 0") {
		t.Fatalf("status = %q", out)
	}
	b.ok(&fake{}, "sync")
	if out := b.ok(&fake{}, "get", "ops/deploy").out; !strings.Contains(out, "make mine") {
		t.Fatalf("remote side = %q", out)
	}
}

func TestSyncQuietSilent(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.init()
	h.ok(&fake{}, "lock")
	h.ok(&fake{}, "remote", "add", bareRemote(t))
	r := h.ok(&fake{}, "sync", "--quiet")
	if r.out != "" || r.err != "" {
		t.Fatalf("output %q %q", r.out, r.err)
	}
	if out := h.ok(&fake{}, "status").out; !strings.Contains(out, "(pushed)") {
		t.Fatalf("status = %q", out)
	}
}

func TestInteractiveStatusResolvesBeforeSync(t *testing.T) {
	t.Parallel()
	a, b := joined(t)
	b.editDeploy("make theirs")
	b.stdin("THEIRPIN\n", "edit", "ops/deploy", "--secret-field", "pin")
	b.ok(&fake{}, "sync")
	a.editDeploy("make mine")
	a.stdin("MYPIN\n", "edit", "ops/deploy", "--secret-field", "pin")
	a.run(&fake{}, "sync")
	f := &fake{picks: []string{keepMine}}
	r := a.ok(f, "status", "-i")
	noLeak(t, r, "THEIRPIN", "MYPIN")
	if !strings.Contains(r.err, `command: mine "make mine", theirs "make theirs"`) || !strings.Contains(r.err, "pin: secret changed") {
		t.Fatalf("diff %q", r.err)
	}
	if slices.Contains(f.prompts, "Sync now?") {
		t.Fatalf("prompts %q", f.prompts)
	}
	if out := a.ok(&fake{}, "status").out; strings.Contains(out, "conflict") {
		t.Fatalf("status = %q", out)
	}
}
