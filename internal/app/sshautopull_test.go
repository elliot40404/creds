package app

import (
	"crypto/sha256"
	"os"
	"path/filepath"
	"testing"

	"github.com/elliot40404/creds/internal/gitsync"
	"github.com/elliot40404/creds/internal/sshdir"
)

func sshPullFlag(t *testing.T, s *Service) bool {
	t.Helper()
	st, err := gitsync.LoadState(s.Paths.State())
	if err != nil {
		t.Fatal(err)
	}
	return st.SSHPull
}

func TestSSHAutoPullWritesOnlySafeChanges(t *testing.T) {
	t.Parallel()
	s, dir := sshService(t)
	setSSHAuto(t, s, "true")
	plantSSHCases(t, s, dir)
	if hint := s.SSH().AutoPull(); hint != "" {
		t.Fatalf("pulled without a sync: %q", hint)
	}
	s.SSH().markPull()
	if hint := s.SSH().AutoPull(); hint != "ssh: 3 files not pulled, run creds ssh pull" {
		t.Fatalf("hint %q", hint)
	}
	want := map[string]string{
		"remote_new": "Host a\n", "remote_change": "Host b\n", "remote_delete": "Host a\n",
		"conflict": "Host b\n", "conflict_new": "Host b\n", "local_changed": "Host b\n",
	}
	for name, body := range want {
		if got := diskSSH(t, dir, "config.d/"+name); got != body {
			t.Errorf("%s: %q, want %q", name, got, body)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "config.d", "local_deleted")); !os.IsNotExist(err) {
		t.Fatalf("local delete undone: %v", err)
	}
	if sshPullFlag(t, s) {
		t.Fatal("flag kept")
	}
	if hint := s.SSH().AutoPull(); hint != "" {
		t.Fatalf("hint shown twice: %q", hint)
	}
}

func TestSSHAutoPullConflictHint(t *testing.T) {
	t.Parallel()
	s, _, dir := pushedSSH(t)
	writeSSHFile(t, dir, "config", []byte("Host vault\n"))
	pushSSH(t, s, SSHPushOptions{})
	st := loadSSHState(t, s)
	fs := st.Files["config"]
	fs.Sum = sha256.Sum256([]byte("Host a\n"))
	st.Files["config"] = fs
	if err := sshdir.SaveState(s.Paths.SSHState(), st); err != nil {
		t.Fatal(err)
	}
	writeSSHFile(t, dir, "config", []byte("Host here\n"))
	s.SSH().markPull()
	if hint := s.SSH().Auto(); hint != "ssh: config changed here and in the vault, run creds ssh pull" {
		t.Fatalf("hint %q", hint)
	}
	if got := diskSSH(t, dir, "config"); got != "Host here\n" {
		t.Fatalf("disk %q", got)
	}
	if got := vaultSSH(t, s, "config"); got != "Host vault\n" {
		t.Fatalf("vault %q", got)
	}
}

func TestSSHAutoPullWaitsForUnlock(t *testing.T) {
	t.Parallel()
	s, fp, dir := pushedSSH(t)
	s.SSH().markPull()
	if err := s.Lock(); err != nil {
		t.Fatal(err)
	}
	fp.prompts, fp.warned = nil, nil
	if hint := s.SSH().AutoPull(); hint != "" {
		t.Fatalf("hint %q", hint)
	}
	if len(fp.prompts) != 0 || len(fp.warned) != 0 || !sshPullFlag(t, s) {
		t.Fatalf("prompts %v warned %v flag %v", fp.prompts, fp.warned, sshPullFlag(t, s))
	}
	if got := diskSSH(t, dir, "config"); got != "Host a\n" {
		t.Fatalf("disk %q", got)
	}
}

func TestSSHAutoPullOffWithoutKnownFiles(t *testing.T) {
	t.Parallel()
	s, _ := knownSSHFiles(t, 0)
	s.SSH().markPull()
	if sshPullFlag(t, s) {
		t.Fatal("flag set with ssh auto off")
	}
	s.SSH().setPull(true)
	if hint := s.SSH().AutoPull(); hint != "" || sshPullFlag(t, s) {
		t.Fatalf("hint %q flag %v", hint, sshPullFlag(t, s))
	}
}
