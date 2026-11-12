package app

import (
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/elliot40404/creds/internal/testutil"
)

type vaultSeed struct {
	id   string
	code string
	fp   *fakePrompter
}

var seeds, vaultCalls sync.Map

func initVault(t *testing.T) (*Service, *fakePrompter, *clock, string) {
	t.Helper()
	name := fmt.Sprintf("init%d", vaultIndex(t))
	src := fixtures.Dir(t, name, func(dir string) { seeds.Store(name, buildSeed(t, dir)) })
	v, _ := seeds.Load(name)
	sd, _ := v.(vaultSeed)
	s, _, c := newService(t)
	fp := sd.fp.clone()
	s.Prompter = fp
	testutil.CopyTree(t, src, s.Paths.Home)
	s.strayChecked = true
	if err := s.saveSession(sd.id); err != nil {
		t.Fatal(err)
	}
	s.checkPerms()
	return s, fp, c, sd.code
}

func vaultIndex(t *testing.T) int32 {
	v, loaded := vaultCalls.LoadOrStore(t, new(atomic.Int32))
	if !loaded {
		t.Cleanup(func() { vaultCalls.Delete(t) })
	}
	n, _ := v.(*atomic.Int32)
	return n.Add(1) - 1
}

func buildSeed(t *testing.T, dir string) vaultSeed {
	t.Helper()
	s, fp, _, code := freshVault(t)
	id, err := s.sessionIdentity()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(s.Paths.Session()); err != nil {
		t.Fatal(err)
	}
	testutil.CopyTree(t, s.Paths.Home, dir)
	return vaultSeed{id: id.String(), code: code, fp: fp.clone()}
}
