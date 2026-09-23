package cli

import (
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/elliot40404/creds/internal/fsutil"
	"github.com/elliot40404/creds/internal/testutil"
)

var fixtures struct {
	dir   string
	mu    sync.Mutex
	built map[string]*fixture
}

type fixture struct {
	once sync.Once
	ok   bool
}

func TestMain(m *testing.M) {
	os.Exit(runWithFixtures(m))
}

func runWithFixtures(m *testing.M) int {
	dir, err := os.MkdirTemp("", "creds-cli-")
	if err != nil {
		return 1
	}
	defer func() { _ = os.RemoveAll(dir) }()
	fixtures.dir = dir
	fixtures.built = map[string]*fixture{}
	return testutil.Run(m)
}

func (h *harness) fromFixture(name string, build func(*harness)) {
	h.t.Helper()
	fixtures.mu.Lock()
	f := fixtures.built[name]
	if f == nil {
		f = &fixture{}
		fixtures.built[name] = f
	}
	fixtures.mu.Unlock()
	src := filepath.Join(fixtures.dir, name)
	f.once.Do(func() {
		build(&harness{t: h.t, home: src, now: h.now, clip: &fakeClip{}})
		f.ok = true
	})
	if !f.ok {
		h.t.Fatalf("fixture %s failed to build", name)
	}
	if err := os.CopyFS(h.home, os.DirFS(src)); err != nil {
		h.t.Fatal(err)
	}
	if err := fsutil.SecureTree(h.home); err != nil {
		h.t.Fatal(err)
	}
}
