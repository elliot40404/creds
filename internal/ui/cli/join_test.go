package cli

import (
	"strings"
	"testing"
)

func TestJoinPasswordRetry(t *testing.T) {
	t.Parallel()
	remote := bareRemote(t)
	a := newHarness(t)
	a.init()
	a.addCommand("ops/deploy", "make deploy")
	a.ok(&fake{}, "remote", "add", remote)
	a.ok(&fake{}, "sync")
	b := newHarness(t)
	r := b.run(&fake{passwords: []string{"wrong-1", "wrong-2", "wrong-3"}}, "join", "--yes", remote)
	if r.code == 0 || !strings.Contains(r.err, "run creds join <url> again") || strings.Contains(r.err, "creds recover") {
		t.Fatalf("join %d %q", r.code, r.err)
	}
	b.ok(&fake{passwords: []string{"wrong-1", mainWord}}, "join", "--yes", remote)
	if out := b.ok(&fake{}, "get", "ops/deploy").out; !strings.Contains(out, "make deploy") {
		t.Fatalf("get = %q", out)
	}
}
