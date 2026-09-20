//go:build unix

package app

import (
	"os"
	"strings"
	"testing"

	"github.com/elliot40404/creds/internal/vaultfiles"
)

func TestWarningsLoosePerms(t *testing.T) {
	t.Parallel()
	s, _, _, _ := initVault(t)
	path := s.vaultFile(vaultfiles.PasswordFile)
	if err := os.Chmod(path, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := s.Unlock(); err != nil {
		t.Fatal(err)
	}
	w := s.Warnings()
	if len(w) != 1 || !strings.Contains(w[0], path) {
		t.Fatalf("warnings %q", w)
	}
	if len(s.Warnings()) != 0 {
		t.Fatal("warnings not drained")
	}
}

func TestUnlockTightensHome(t *testing.T) {
	t.Parallel()
	s, _, _, _ := initVault(t)
	if err := os.Chmod(s.Paths.Home, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := s.Unlock(); err != nil {
		t.Fatal(err)
	}
	if w := s.Warnings(); len(w) != 0 {
		t.Fatalf("warnings %q", w)
	}
	info, err := os.Stat(s.Paths.Home)
	if err != nil || info.Mode().Perm() != 0o700 {
		t.Fatalf("home mode %v %v", info, err)
	}
}

func TestLooseSessionIsDropped(t *testing.T) {
	t.Parallel()
	s, fp, _, _ := initVault(t)
	if err := s.Unlock(); err != nil {
		t.Fatal(err)
	}
	s.Warnings()
	path := s.Paths.Session()
	if err := os.Chmod(path, 0o644); err != nil {
		t.Fatal(err)
	}
	fp.passwords = tries(mainWord)
	if err := s.Unlock(); err != nil {
		t.Fatal(err)
	}
	if len(fp.passwords) == len(tries(mainWord)) {
		t.Fatal("loose session reused without a password")
	}
	w := s.Warnings()
	if !strings.Contains(strings.Join(w, " "), "dropped unprotected session") {
		t.Fatalf("warnings %q", w)
	}
}

func TestWarningsRecheckedEveryUnlock(t *testing.T) {
	t.Parallel()
	s, fp, _, _ := initVault(t)
	if err := s.Unlock(); err != nil {
		t.Fatal(err)
	}
	if w := s.Warnings(); len(w) != 0 {
		t.Fatalf("warnings %q", w)
	}
	path := s.vaultFile(vaultfiles.RecoveryFile)
	if err := os.Chmod(path, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := unlockWith(t, s, fp, mainWord); err != nil {
		t.Fatal(err)
	}
	w := s.Warnings()
	if len(w) != 1 || !strings.Contains(w[0], path) {
		t.Fatalf("warnings %q", w)
	}
}

func TestWarningsNotRepeatedWithinOneDrain(t *testing.T) {
	t.Parallel()
	s, fp, _, _ := initVault(t)
	path := s.vaultFile(vaultfiles.PasswordFile)
	if err := os.Chmod(path, 0o644); err != nil {
		t.Fatal(err)
	}
	for range 3 {
		if err := unlockWith(t, s, fp, mainWord); err != nil {
			t.Fatal(err)
		}
	}
	if w := s.Warnings(); len(w) != 1 {
		t.Fatalf("warnings %q", w)
	}
}

func TestSyncQuietDropsLooseSession(t *testing.T) {
	t.Parallel()
	s, _, _, _ := initVault(t)
	withRemote(t, s)
	path := s.Paths.Session()
	if err := os.Chmod(path, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := s.SyncQuiet(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("loose session kept by background sync")
	}
}
