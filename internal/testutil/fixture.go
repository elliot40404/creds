package testutil

import (
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/elliot40404/creds/internal/fsutil"
)

type Fixtures struct {
	dir   string
	mu    sync.Mutex
	built map[string]*fixture
}

type fixture struct {
	once sync.Once
	ok   bool
}

func (f *Fixtures) Main(m *testing.M) {
	os.Exit(f.run(m))
}

func (f *Fixtures) run(m *testing.M) int {
	dir, err := os.MkdirTemp("", "creds-fixture-")
	if err != nil {
		return 1
	}
	defer func() { _ = os.RemoveAll(dir) }()
	f.dir = dir
	f.built = map[string]*fixture{}
	return Run(m)
}

func (f *Fixtures) Dir(tb testing.TB, name string, build func(dir string)) string {
	tb.Helper()
	fx := f.get(name)
	dir := filepath.Join(f.dir, name)
	fx.once.Do(func() {
		build(dir)
		fx.ok = true
	})
	if !fx.ok {
		tb.Fatalf("fixture %s failed to build", name)
	}
	return dir
}

func (f *Fixtures) get(name string) *fixture {
	f.mu.Lock()
	defer f.mu.Unlock()
	fx := f.built[name]
	if fx == nil {
		fx = &fixture{}
		f.built[name] = fx
	}
	return fx
}

func CopyTree(tb testing.TB, src, dst string) {
	tb.Helper()
	if err := os.CopyFS(dst, os.DirFS(src)); err != nil {
		tb.Fatal(err)
	}
	if err := fsutil.SecureTree(dst); err != nil {
		tb.Fatal(err)
	}
}
