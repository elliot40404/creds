package app

import (
	"crypto/sha256"
	"errors"
	"maps"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/elliot40404/creds/internal/sshdir"
	"github.com/elliot40404/creds/internal/vault"
)

type sshCase struct {
	disk, remote, base string
	want               SSHChange
}

var sshCases = map[string]sshCase{
	"config.d/same":          {"Host a\n", "Host a\n", "Host a\n", SSHSame},
	"config.d/same_nobase":   {"Host a\n", "Host a\n", "", SSHSame},
	"config.d/local_new":     {"Host a\n", "", "", SSHLocalNew},
	"config.d/remote_new":    {"", "Host a\n", "", SSHRemoteNew},
	"config.d/local_changed": {"Host b\n", "Host a\n", "Host a\n", SSHLocalChanged},
	"config.d/remote_change": {"Host a\n", "Host b\n", "Host a\n", SSHRemoteChanged},
	"config.d/local_deleted": {"", "Host a\n", "Host a\n", SSHLocalDeleted},
	"config.d/remote_delete": {"Host a\n", "", "Host a\n", SSHRemoteDeleted},
	"config.d/conflict":      {"Host b\n", "Host c\n", "Host a\n", SSHConflict},
	"config.d/conflict_new":  {"Host b\n", "Host c\n", "", SSHConflict},
	"config.d/gone":          {"", "", "Host a\n", 0},
}

func sshService(t *testing.T) (*Service, string) {
	t.Helper()
	s, _, _, _ := initVault(t)
	return s, filepath.Join(s.UserHome, ".ssh")
}

func plantSSHCases(t *testing.T, s *Service, dir string) {
	t.Helper()
	st := sshdir.State{Files: map[string]sshdir.FileState{}}
	for rel, c := range sshCases {
		if c.disk != "" {
			writeSSHFile(t, dir, rel, []byte(c.disk))
		}
		if c.remote != "" {
			addSSHEntry(t, s, sshdir.File{Rel: rel, Kind: sshdir.KindConfig, Data: []byte(c.remote)})
		}
		if c.base != "" {
			st.Files[rel] = sshdir.FileState{Sum: sha256.Sum256([]byte(c.base))}
		}
	}
	if err := sshdir.SaveState(s.Paths.SSHState(), st); err != nil {
		t.Fatal(err)
	}
}

func addSSHEntry(t *testing.T, s *Service, f sshdir.File) {
	t.Helper()
	e, err := SSHEntry(f)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Add(e); err != nil {
		t.Fatal(err)
	}
}

func byRel(st SSHStatus) map[string]SSHFileStatus {
	out := map[string]SSHFileStatus{}
	for _, f := range st.Files {
		out[f.Rel] = f
	}
	return out
}

func TestSSHStatusEveryCase(t *testing.T) {
	t.Parallel()
	s, dir := sshService(t)
	plantSSHCases(t, s, dir)
	st, err := s.SSHStatus()
	if err != nil {
		t.Fatal(err)
	}
	got := byRel(st)
	for rel, c := range sshCases {
		f, ok := got[rel]
		switch {
		case c.want == 0 && ok:
			t.Errorf("%s: listed as %s", rel, f.Change)
		case c.want == 0:
		case !ok:
			t.Errorf("%s: not listed", rel)
		case f.Change != c.want || f.Kind != sshdir.KindConfig:
			t.Errorf("%s: got %s kind %d, want %s", rel, f.Change, f.Kind, c.want)
		case (f.Disk != nil) != (c.disk != "") || (f.Vault != nil) != (c.remote != ""):
			t.Errorf("%s: disk %v vault %v", rel, f.Disk != nil, f.Vault != nil)
		}
	}
	if !slices.IsSortedFunc(st.Files, func(a, b SSHFileStatus) int { return strings.Compare(a.Rel, b.Rel) }) {
		t.Error("files not sorted by path")
	}
}

func TestSSHStatusKeysAndNotes(t *testing.T) {
	t.Parallel()
	s, dir := sshService(t)
	priv, pub := sshKeyPair(t)
	writeSSHFile(t, dir, "config", []byte("IdentityFile ~/.ssh/gone\nIdentityFile %d/x\n"))
	writeSSHFile(t, dir, "id_ed25519", priv)
	writeSSHFile(t, dir, "id_ed25519.pub", pub)
	writeSSHFile(t, dir, "work_key", priv)
	writeSSHFile(t, dir, "config.d/big", []byte(strings.Repeat("a", sshdir.MaxFileSize+1)))
	addSSHEntry(t, s, sshdir.File{Rel: "id_ed25519", Kind: sshdir.KindPrivateKey, Data: priv})
	if err := sshdir.SaveState(s.Paths.SSHState(), sshdir.State{Skipped: []string{"work_key"}}); err != nil {
		t.Fatal(err)
	}
	st, err := s.SSHStatus()
	if err != nil {
		t.Fatal(err)
	}
	got := byRel(st)
	want := map[string]SSHFileStatus{
		"config":         {Kind: sshdir.KindConfig, Change: SSHLocalNew},
		"id_ed25519":     {Kind: sshdir.KindPrivateKey, Change: SSHSame},
		"id_ed25519.pub": {Kind: sshdir.KindPublicKey, Change: SSHLocalNew},
		"work_key":       {Kind: sshdir.KindPrivateKey, Change: SSHLocalNew, Unused: true, Skipped: true},
	}
	if !slices.Equal(slices.Sorted(maps.Keys(got)), slices.Sorted(maps.Keys(want))) {
		t.Fatalf("files %v", slices.Sorted(maps.Keys(got)))
	}
	for rel, w := range want {
		f := got[rel]
		if f.Kind != w.Kind || f.Change != w.Change || f.Unused != w.Unused || f.Skipped != w.Skipped {
			t.Errorf("%s: got %+v", rel, f)
		}
	}
	checkSSHNotes(t, st, dir)
}

func checkSSHNotes(t *testing.T, st SSHStatus, dir string) {
	t.Helper()
	if st.Dir != dir {
		t.Errorf("dir %q, want %q", st.Dir, dir)
	}
	if !slices.Equal(st.Missing, []string{"gone"}) {
		t.Errorf("missing %v", st.Missing)
	}
	if len(st.Warnings) != 1 || !errors.Is(st.Warnings[0], sshdir.ErrToken) {
		t.Errorf("warnings %v", st.Warnings)
	}
	if len(st.Skipped) != 1 || st.Skipped[0].Rel != "config.d/big" {
		t.Errorf("skipped %v", st.Skipped)
	}
}

func TestSSHStatusNoSSHDir(t *testing.T) {
	t.Parallel()
	s, _ := sshService(t)
	addSSHEntry(t, s, sshdir.File{Rel: "config", Kind: sshdir.KindConfig, Data: []byte("Host a\n")})
	st, err := s.SSHStatus()
	if err != nil {
		t.Fatal(err)
	}
	if len(st.Files) != 1 || st.Files[0].Change != SSHRemoteNew || st.Files[0].Disk != nil {
		t.Fatalf("files %+v", st.Files)
	}
	if string(st.Files[0].Vault.Data) != "Host a\n" {
		t.Fatalf("vault data %q", st.Files[0].Vault.Data)
	}
}

func TestSSHStatusListsBadEntries(t *testing.T) {
	t.Parallel()
	s, _ := sshService(t)
	bad := vault.Entry{Path: "ssh/tunnel", Type: vault.TypeSSHFile, Fields: []vault.Field{{Name: FieldContent, Value: "x"}}}
	if _, err := s.Add(bad); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Add(vault.Entry{Path: "ssh/server", Type: vault.TypeNote}); err != nil {
		t.Fatal(err)
	}
	st, err := s.SSHStatus()
	if err != nil {
		t.Fatal(err)
	}
	if len(st.Files) != 0 || len(st.Invalid) != 1 || !errors.Is(st.Invalid[0], ErrSSHFile) {
		t.Fatalf("files %+v invalid %v", st.Files, st.Invalid)
	}
}

func TestSSHStatusErrors(t *testing.T) {
	t.Parallel()
	s, dir := sshService(t)
	writeSSHFile(t, dir, "config", []byte("Host a\n"))
	writeSSHFile(t, s.Paths.Home, "ssh.json", []byte("{"))
	if _, err := s.SSHStatus(); !errors.Is(err, sshdir.ErrStateCorrupt) {
		t.Fatalf("corrupt state: %v", err)
	}
	if err := s.Lock(); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SSHStatus(); !errors.Is(err, errEmpty) {
		t.Fatalf("locked: %v", err)
	}
}

func TestSSHStateIsAHomeFile(t *testing.T) {
	t.Parallel()
	s, _, _ := newService(t)
	if !slices.Contains(s.homeFiles(), s.Paths.SSHState()) {
		t.Fatal("ssh.json not cleaned with the other home files")
	}
}
