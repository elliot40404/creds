package app

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"slices"
	"testing"
	"time"

	"github.com/elliot40404/creds/internal/crypto"
	"github.com/elliot40404/creds/internal/format"
	"github.com/elliot40404/creds/internal/gitsync"
	"github.com/elliot40404/creds/internal/vaultfiles"
)

func unlockWith(t *testing.T, s *Service, fp *fakePrompter, pw string) error {
	t.Helper()
	if err := s.Lock(); err != nil {
		t.Fatal(err)
	}
	fp.passwords = tries(pw)
	err := s.Unlock()
	fp.passwords = nil
	noSecret(t, err, pw)
	return err
}

func tries(secret string) []string {
	return slices.Repeat([]string{secret}, MaxTries)
}

func TestUnlockViaSession(t *testing.T) {
	t.Parallel()
	s, fp, _, _ := initVault(t)
	fp.prompts = nil
	if err := s.Unlock(); err != nil {
		t.Fatal(err)
	}
	if len(fp.prompts) != 0 {
		t.Fatal("prompted with live session")
	}
}

func TestUnlockViaPassword(t *testing.T) {
	t.Parallel()
	s, fp, _, _ := initVault(t)
	if err := unlockWith(t, s, fp, mainWord); err != nil {
		t.Fatal(err)
	}
	if err := s.Unlock(); err != nil {
		t.Fatalf("session not saved: %v", err)
	}
}

func TestUnlockExpiredSession(t *testing.T) {
	t.Parallel()
	s, fp, c, _ := initVault(t)
	expire(s, c)
	fp.passwords = []string{mainWord}
	if err := s.Unlock(); err != nil {
		t.Fatal(err)
	}
	if len(fp.passwords) != 0 {
		t.Fatal("password not asked")
	}
}

func TestUnlockWrongPassword(t *testing.T) {
	t.Parallel()
	s, fp, _, _ := initVault(t)
	if err := unlockWith(t, s, fp, nextWord); !errors.Is(err, crypto.ErrWrongSecret) {
		t.Fatalf("err = %v", err)
	}
	fp.passwords = nil
	if err := s.Unlock(); !errors.Is(err, errEmpty) {
		t.Fatalf("session saved after wrong password: %v", err)
	}
}

func TestNoVaultBeforePrompt(t *testing.T) {
	t.Parallel()
	s, fp, _ := newService(t)
	ops := map[string]func() error{
		"unlock":  s.Unlock,
		"recover": s.Recover,
		"passwd":  s.Passwd,
		"get": func() error {
			_, err := s.Get("a")
			return err
		},
	}
	for name, op := range ops {
		fp.passwords = []string{mainWord}
		if err := op(); !errors.Is(err, ErrNoVault) {
			t.Fatalf("%s: err = %v", name, err)
		}
		if len(fp.prompts) != 0 {
			t.Fatalf("%s: prompted %v", name, fp.prompts)
		}
	}
}

func TestRecover(t *testing.T) {
	t.Parallel()
	s, fp, _, code := initVault(t)
	if err := s.Lock(); err != nil {
		t.Fatal(err)
	}
	fp.passwords = []string{code, nextWord, nextWord}
	if err := s.Recover(); err != nil {
		t.Fatal(err)
	}
	if err := unlockWith(t, s, fp, mainWord); !errors.Is(err, crypto.ErrWrongSecret) {
		t.Fatalf("old password: %v", err)
	}
	if err := unlockWith(t, s, fp, nextWord); err != nil {
		t.Fatalf("new password: %v", err)
	}
}

func TestRecoverWrongCode(t *testing.T) {
	t.Parallel()
	s, fp, _, _ := initVault(t)
	fp.passwords = tries("AAAAA-BBBBB-CCCCC-DDDDD-EEEEE-FFFFF")
	if err := s.Recover(); !errors.Is(err, crypto.ErrWrongSecret) {
		t.Fatalf("err = %v", err)
	}
}

func TestPasswd(t *testing.T) {
	t.Parallel()
	s, fp, _, code := initVault(t)
	fp.passwords = []string{mainWord, nextWord, nextWord}
	if err := s.Passwd(); err != nil {
		t.Fatal(err)
	}
	if err := unlockWith(t, s, fp, mainWord); !errors.Is(err, crypto.ErrWrongSecret) {
		t.Fatalf("old password: %v", err)
	}
	if err := unlockWith(t, s, fp, nextWord); err != nil {
		t.Fatalf("new password: %v", err)
	}
	fp.passwords = []string{code, mainWord, mainWord}
	if err := s.Recover(); err != nil {
		t.Fatalf("recovery after passwd: %v", err)
	}
}

func TestPasswdWrongOld(t *testing.T) {
	t.Parallel()
	s, fp, _, _ := initVault(t)
	fp.passwords = tries(nextWord)
	err := s.Passwd()
	if !errors.Is(err, crypto.ErrWrongSecret) {
		t.Fatalf("err = %v", err)
	}
	noSecret(t, err, nextWord)
}

func TestUnlockMigratesV0(t *testing.T) {
	t.Parallel()
	s, fp, _, _ := initVault(t)
	if _, err := s.Add(dbEntry("prod/db")); err != nil {
		t.Fatal(err)
	}
	meta, err := format.LoadMeta(s.vaultFile(vaultfiles.MetaFile))
	if err != nil {
		t.Fatal(err)
	}
	v0 := fmt.Sprintf(`{"recipient":%q,"created_at":%q}`, meta.Recipient, meta.Created.Format(time.RFC3339))
	if err := os.WriteFile(s.vaultFile(vaultfiles.MetaFile), []byte(v0), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := unlockWith(t, s, fp, mainWord); err != nil {
		t.Fatal(err)
	}
	list, err := s.List()
	if err != nil || len(list) != 1 || list[0].Path != "prod/db" {
		t.Fatalf("list = %v %v", list, err)
	}
	got, err := format.LoadMeta(s.vaultFile(vaultfiles.MetaFile))
	if err != nil || got.FormatVersion != format.CurrentVersion {
		t.Fatalf("meta = %+v %v", got, err)
	}
}

func TestUnlockRefusesOversizedIdentity(t *testing.T) {
	t.Parallel()
	s, fp, _, _ := initVault(t)
	big := make([]byte, maxIdentitySize+1)
	if err := os.WriteFile(s.vaultFile(vaultfiles.PasswordFile), big, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := unlockWith(t, s, fp, mainWord); err == nil {
		t.Fatal("want error")
	}
}

func TestUnlockRefusesIdentityDir(t *testing.T) {
	t.Parallel()
	s, fp, _, _ := initVault(t)
	path := s.vaultFile(vaultfiles.PasswordFile)
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := unlockWith(t, s, fp, mainWord); err == nil {
		t.Fatal("want error")
	}
}

func TestPasswordRetry(t *testing.T) {
	t.Parallel()
	s, fp, _, code := initVault(t)
	ops := []struct {
		name   string
		op     func() error
		wrong  string
		right  []string
		prompt string
	}{
		{"unlock", s.Unlock, nextWord, []string{mainWord}, "Master password"},
		{"passwd", s.Passwd, nextWord, []string{mainWord, mainWord, mainWord}, "Current master password"},
		{"recover", s.Recover, "AAAAA-BBBBB-CCCCC-DDDDD-EEEEE-FFFFF", []string{code, mainWord, mainWord}, "Recovery code"},
	}
	for _, o := range ops {
		if err := s.Lock(); err != nil {
			t.Fatal(err)
		}
		fp.warned, fp.prompts = nil, nil
		fp.passwords = append(tries(o.wrong)[1:], o.right...)
		if err := o.op(); err != nil {
			t.Fatalf("%s: %v", o.name, err)
		}
		if len(fp.warned) != MaxTries-1 || fp.prompts[MaxTries-1] != o.prompt {
			t.Fatalf("%s: warned %q prompts %q", o.name, fp.warned, fp.prompts)
		}
	}
}

func TestPasswdWaitsForSyncLock(t *testing.T) {
	t.Parallel()
	s, fp, c, _ := initVault(t)
	if err := gitsync.SaveState(s.Paths.State(), gitsync.State{PasswordChanged: c.t}); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(s.vaultFile(vaultfiles.PasswordFile))
	if err != nil {
		t.Fatal(err)
	}
	lock, err := gitsync.AcquireLock(s.Paths.SyncLock(), c.t)
	if err != nil {
		t.Fatal(err)
	}
	s.Waiting = func() {
		if now, err := os.ReadFile(s.vaultFile(vaultfiles.PasswordFile)); err != nil || !bytes.Equal(now, before) {
			t.Errorf("password file changed while the sync lock was held: %v", err)
		}
		if err := lock.Release(); err != nil {
			t.Error(err)
		}
	}
	fp.passwords = []string{mainWord, nextWord, nextWord}
	if err := s.Passwd(); err != nil {
		t.Fatal(err)
	}
	st, err := gitsync.LoadState(s.Paths.State())
	if err != nil || !st.PasswordChanged.IsZero() {
		t.Fatalf("password change not acknowledged: %+v %v", st, err)
	}
	if gitsync.Locked(s.Paths.SyncLock(), c.t) {
		t.Fatal("lock not released")
	}
}
