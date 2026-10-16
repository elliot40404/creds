package app

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/elliot40404/creds/internal/config"
	"github.com/elliot40404/creds/internal/sshdir"
)

func knownSSHFiles(tb testing.TB, n int) (*Service, string) {
	tb.Helper()
	s := &Service{Paths: config.Paths{Home: tb.TempDir()}, Config: config.Default(), Prompter: &fakePrompter{}, UserHome: tb.TempDir()}
	dir := filepath.Join(s.UserHome, ".ssh")
	st := sshdir.State{Files: map[string]sshdir.FileState{}}
	for i := range n {
		f := sshdir.File{Rel: fmt.Sprintf("config.d/host%d", i), Kind: sshdir.KindConfig, Data: fmt.Appendf(nil, "Host h%d\n", i)}
		p := filepath.Join(dir, filepath.FromSlash(f.Rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
			tb.Fatal(err)
		}
		if err := os.WriteFile(p, f.Data, 0o600); err != nil {
			tb.Fatal(err)
		}
		st.Files[f.Rel] = sshFileState(dir, f)
	}
	if err := sshdir.SaveState(s.Paths.SSHState(), st); err != nil {
		tb.Fatal(err)
	}
	return s, dir
}

func pushedSSH(t *testing.T) (*Service, *fakePrompter, string) {
	t.Helper()
	s, fp, _, _ := initVault(t)
	dir := filepath.Join(s.UserHome, ".ssh")
	writeSSHFile(t, dir, "config", []byte("Host a\n"))
	writeSSHFile(t, dir, "config.d/b", []byte("Host b\n"))
	pushSSH(t, s, SSHPushOptions{})
	return s, fp, dir
}

func TestSSHAutoPushesAChangedKnownFile(t *testing.T) {
	t.Parallel()
	s, _, dir := pushedSSH(t)
	writeSSHFile(t, dir, "config", []byte("Host a\n  Port 2222\n"))
	writeSSHFile(t, dir, "config.d/new", []byte("Host n\n"))
	if hint := s.SSHAutoPush(); hint != "" {
		t.Fatalf("hint %q", hint)
	}
	if got := vaultSSH(t, s, "config"); got != "Host a\n  Port 2222\n" {
		t.Fatalf("vault config %q", got)
	}
	st := loadSSHState(t, s)
	if st.Files["config"].Sum != sha256.Sum256([]byte("Host a\n  Port 2222\n")) || st.Files["config"].Size == 0 {
		t.Fatalf("state %+v", st.Files["config"])
	}
	if _, ok := st.Files["config.d/new"]; ok {
		t.Fatal("new file pushed")
	}
	if _, err := s.Get(SSHPrefix + "config.d/new"); err == nil {
		t.Fatal("new file in vault")
	}
}

func TestSSHAutoUnchangedOpensNoVault(t *testing.T) {
	t.Parallel()
	s, dir := knownSSHFiles(t, 3)
	if hint := s.SSHAutoPush(); hint != "" {
		t.Fatalf("hint %q", hint)
	}
	writeSSHFile(t, dir, "config.d/host1", []byte("Host changed\n"))
	if hint := s.SSHAutoPush(); hint != "ssh: auto push failed, run creds ssh push" {
		t.Fatalf("changed file did not reach the vault: %q", hint)
	}
}

func TestSSHAutoRefreshesStatWhenOnlyTimeChanged(t *testing.T) {
	t.Parallel()
	s, _, dir := pushedSSH(t)
	before := loadSSHState(t, s).Files["config"]
	later := before.ModTime.Add(1e9)
	if err := os.Chtimes(filepath.Join(dir, "config"), later, later); err != nil {
		t.Fatal(err)
	}
	if hint := s.SSHAutoPush(); hint != "" {
		t.Fatalf("hint %q", hint)
	}
	after := loadSSHState(t, s).Files["config"]
	if after.Sum != before.Sum || !after.ModTime.Equal(later) {
		t.Fatalf("state %+v, want mtime %v", after, later)
	}
	if dirty := sshStatDirty(dir, loadSSHState(t, s)); len(dirty) != 0 {
		t.Fatalf("dirty %v", dirty)
	}
}

func TestSSHAutoConflictHint(t *testing.T) {
	t.Parallel()
	s, dir := sshService(t)
	writeSSHFile(t, dir, "config", []byte("Host b\n"))
	addSSHEntry(t, s, sshdir.File{Rel: "config", Kind: sshdir.KindConfig, Data: []byte("Host c\n")})
	st := sshdir.State{Files: map[string]sshdir.FileState{"config": {Sum: sha256.Sum256([]byte("Host a\n"))}}}
	if err := sshdir.SaveState(s.Paths.SSHState(), st); err != nil {
		t.Fatal(err)
	}
	if hint := s.SSHAutoPush(); hint != "ssh: config changed here and in the vault, run creds ssh status" {
		t.Fatalf("hint %q", hint)
	}
	if got := vaultSSH(t, s, "config"); got != "Host c\n" {
		t.Fatalf("vault config %q", got)
	}
}

func TestSSHAutoEveryCase(t *testing.T) {
	t.Parallel()
	s, dir := sshService(t)
	plantSSHCases(t, s, dir)
	if hint := s.SSHAutoPush(); hint != "ssh: 4 files not pushed, run creds ssh status" {
		t.Fatalf("hint %q", hint)
	}
	if got := vaultSSH(t, s, "config.d/local_changed"); got != "Host b\n" {
		t.Fatalf("local change not pushed: %q", got)
	}
	if got := vaultSSH(t, s, "config.d/conflict"); got != "Host c\n" {
		t.Fatalf("conflict pushed: %q", got)
	}
	if _, err := s.Get(SSHPrefix + "config.d/local_new"); err == nil {
		t.Fatal("local new pushed")
	}
}

func TestSSHAutoOffWhenLocked(t *testing.T) {
	t.Parallel()
	s, fp, dir := pushedSSH(t)
	if err := s.Lock(); err != nil {
		t.Fatal(err)
	}
	writeSSHFile(t, dir, "config", []byte("Host locked\n"))
	before := loadSSHState(t, s)
	fp.prompts, fp.warned = nil, nil
	if hint := s.SSHAutoPush(); hint != "" {
		t.Fatalf("hint %q", hint)
	}
	if len(fp.prompts) != 0 || len(fp.warned) != 0 {
		t.Fatalf("prompts %v warned %v", fp.prompts, fp.warned)
	}
	if !sameSSHState(before, loadSSHState(t, s)) {
		t.Fatal("ssh.json changed")
	}
	fp.passwords = []string{mainWord}
	if got := vaultSSH(t, s, "config"); got != "Host a\n" {
		t.Fatalf("vault config %q", got)
	}
}

func TestSSHAutoOffWithoutKnownFiles(t *testing.T) {
	t.Parallel()
	s, dir := knownSSHFiles(t, 0)
	writeSSHFile(t, dir, "config", []byte("Host a\n"))
	if hint := s.SSHAutoPush(); hint != "" {
		t.Fatalf("hint %q", hint)
	}
	if err := sshdir.SaveState(s.Paths.SSHState(), sshdir.State{Skipped: []string{"config"}}); err != nil {
		t.Fatal(err)
	}
	if hint := s.SSHAutoPush(); hint != "" {
		t.Fatalf("hint %q", hint)
	}
}

func BenchmarkSSHAutoPushUnchanged(b *testing.B) {
	s, _ := knownSSHFiles(b, 30)
	b.ReportAllocs()
	for b.Loop() {
		if hint := s.SSHAutoPush(); hint != "" {
			b.Fatal(hint)
		}
	}
}
