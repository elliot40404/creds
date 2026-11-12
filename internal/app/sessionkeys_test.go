package app

import (
	"encoding/json"
	"errors"
	"os"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/elliot40404/creds/internal/config"
	"github.com/elliot40404/creds/internal/keystore"
	"github.com/elliot40404/creds/internal/testutil"
)

func protectedMethod() string {
	if runtime.GOOS == "windows" {
		return "dpapi"
	}
	return "fake"
}

func sessionRecord(t *testing.T, s *Service) map[string]any {
	t.Helper()
	data, err := os.ReadFile(s.Paths.Session())
	if err != nil {
		t.Fatal(err)
	}
	var r map[string]any
	if err := json.Unmarshal(data, &r); err != nil {
		t.Fatal(err)
	}
	return r
}

func withKeys(s *Service, keys keystore.Store) *Service {
	s.Keystore = func() (keystore.Store, error) { return keys, nil }
	return s.fresh()
}

func relock(t *testing.T, s *Service, fp *fakePrompter) {
	t.Helper()
	if err := s.Lock(); err != nil {
		t.Fatal(err)
	}
	fp.passwords = []string{mainWord}
	if err := s.Unlock(); err != nil {
		t.Fatal(err)
	}
}

func TestUnlockSealsSession(t *testing.T) {
	s, fp, _, _ := initVault(t)
	r := sessionRecord(t, s)
	if r["method"] != protectedMethod() || r["identity"] != nil || r["sealed"] == nil {
		t.Fatalf("record %v", r)
	}
	prompts := len(fp.prompts)
	again := s.fresh()
	if err := again.Unlock(); err != nil || len(fp.prompts) != prompts {
		t.Fatalf("sealed session not reused: %v prompts %v", err, fp.prompts)
	}
	if slices.Contains(s.Warnings(), plainSessionWarning) {
		t.Fatal("plain warning with a key store")
	}
	note := again.PathReport().Paths[3].Note
	if !strings.HasPrefix(note, protectedMethod()+", ") || !strings.HasSuffix(note, " left") {
		t.Fatalf("session note %q", note)
	}
}

func TestProtectOffWritesPlainAndForgets(t *testing.T) {
	s, fp, _, _ := initVault(t)
	keys := &testutil.FakeKeys{}
	on := withKeys(s, keys)
	relock(t, on, fp)
	if r := sessionRecord(t, on); r["method"] != "fake" {
		t.Fatalf("record %v", r)
	}
	cfg, err := config.Set(on.Config, "session.protect", "false")
	if err != nil {
		t.Fatal(err)
	}
	on.Config = cfg
	off := on.fresh()
	if err := off.Unlock(); err != nil {
		t.Fatal(err)
	}
	forgot := len(keys.Forgotten())
	relock(t, off, fp)
	if r := sessionRecord(t, off); r["method"] != "plain" || r["identity"] == nil || keys.Seals() != 1 {
		t.Fatalf("record %v seals %d", r, keys.Seals())
	}
	if len(keys.Forgotten()) != forgot+1 || slices.Contains(off.Warnings(), plainSessionWarning) {
		t.Fatalf("forgets %v", keys.Forgotten())
	}
}

func TestNoKeystoreWarnsAndWritesPlain(t *testing.T) {
	s, fp, _, _ := initVault(t)
	s.Keystore = testutil.NoKeystore
	plain := s.fresh()
	relock(t, plain, fp)
	if r := sessionRecord(t, plain); r["method"] != "plain" {
		t.Fatalf("record %v", r)
	}
	w := plain.Warnings()
	if strings.Count(strings.Join(w, "|"), plainSessionWarning) != 1 {
		t.Fatalf("warnings %v", w)
	}
	if !strings.Contains(plain.PathReport().Paths[3].Note, "plain, ") {
		t.Fatalf("note %q", plain.PathReport().Paths[3].Note)
	}
}

func TestSealFailureWarnsOnceAndStopsRetrying(t *testing.T) {
	s, fp, _, _ := initVault(t)
	keys := &testutil.FakeKeys{SealErr: errors.New("boom")}
	failing := withKeys(s, keys)
	relock(t, failing, fp)
	if r := sessionRecord(t, failing); r["method"] != "plain" {
		t.Fatalf("record %v", r)
	}
	if !slices.Contains(failing.Warnings(), plainSessionWarning) {
		t.Fatal("no plain warning")
	}
	if err := failing.Unlock(); err != nil || keys.Seals() != 1 {
		t.Fatalf("seals %d %v", keys.Seals(), err)
	}
}
