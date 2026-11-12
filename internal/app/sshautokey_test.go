package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/elliot40404/creds/internal/config"
	"github.com/elliot40404/creds/internal/sshdir"
)

func sshAutoSaved(t *testing.T, s *Service) *bool {
	t.Helper()
	c, err := config.Load(s.Paths.Config())
	if err != nil {
		t.Fatal(err)
	}
	if c.SSH.AutoOn() != s.CurrentConfig().SSH.AutoOn() {
		t.Fatalf("saved %+v, loaded %+v", c.SSH, s.CurrentConfig().SSH)
	}
	return c.SSH.Auto
}

func TestSSHPushTurnsOnAuto(t *testing.T) {
	t.Parallel()
	s, _, _ := pushedSSH(t)
	if on := sshAutoSaved(t, s); on == nil || !*on {
		t.Fatalf("ssh.auto %v after the first push", on)
	}
}

func TestSSHPullTurnsOnAuto(t *testing.T) {
	t.Parallel()
	s, dir := sshService(t)
	addSSHEntry(t, s, sshdir.File{Rel: "config", Kind: sshdir.KindConfig, Data: []byte("Host a\n")})
	if _, err := s.SSH().Pull(SSHPullOptions{}); err != nil {
		t.Fatal(err)
	}
	if got := diskSSH(t, dir, "config"); got != "Host a\n" {
		t.Fatalf("disk %q", got)
	}
	if on := sshAutoSaved(t, s); on == nil || !*on {
		t.Fatalf("ssh.auto %v after the first pull", on)
	}
}

func TestSSHAutoNotTurnedOnWithoutFiles(t *testing.T) {
	t.Parallel()
	s, _ := sshService(t)
	pushSSH(t, s, SSHPushOptions{})
	if s.CurrentConfig().SSH.Auto != nil {
		t.Fatalf("ssh.auto set with nothing pushed: %v", *s.CurrentConfig().SSH.Auto)
	}
}

func TestSSHAutoOffStaysOffAndDoesNothing(t *testing.T) {
	t.Parallel()
	s, _, dir := pushedSSH(t)
	setSSHAuto(t, s, "false")
	writeSSHFile(t, dir, "config", []byte("Host a\n  Port 22\n"))
	pushSSH(t, s, SSHPushOptions{})
	if on := sshAutoSaved(t, s); on == nil || *on {
		t.Fatalf("push flipped an explicit false: %v", on)
	}
	writeSSHFile(t, dir, "config", []byte("Host a\n  Port 2222\n"))
	if hint := s.SSHAutoPush(); hint != "" {
		t.Fatalf("hint %q", hint)
	}
	if got := vaultSSH(t, s, "config"); got != "Host a\n  Port 22\n" {
		t.Fatalf("auto pushed while off: %q", got)
	}
	addSSHEntry(t, s, sshdir.File{Rel: "config.d/c", Kind: sshdir.KindConfig, Data: []byte("Host c\n")})
	s.markSSHPull()
	if sshPullFlag(t, s) {
		t.Fatal("flag set while off")
	}
	s.setSSHPull(true)
	if hint := s.SSHAutoPull(); hint != "" || sshPullFlag(t, s) {
		t.Fatalf("hint %q flag %v", hint, sshPullFlag(t, s))
	}
	if _, err := os.Stat(filepath.Join(dir, "config.d", "c")); !os.IsNotExist(err) {
		t.Fatalf("auto pulled while off: %v", err)
	}
}
