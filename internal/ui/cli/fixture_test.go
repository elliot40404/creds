package cli

import (
	"testing"

	"github.com/elliot40404/creds/internal/testutil"
)

var fixtures testutil.Fixtures

func TestMain(m *testing.M) {
	fixtures.Main(m)
}

func (h *harness) fromFixture(name string, build func(*harness)) {
	h.t.Helper()
	src := fixtures.Dir(h.t, name, func(dir string) {
		build(&harness{t: h.t, home: dir, now: h.now, clip: &fakeClip{}, keys: testutil.NoKeystore})
	})
	testutil.CopyTree(h.t, src, h.home)
}
