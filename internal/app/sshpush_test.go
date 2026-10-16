package app

import (
	"crypto/sha256"
	"errors"
	"slices"
	"testing"

	"github.com/elliot40404/creds/internal/sshdir"
	"github.com/elliot40404/creds/internal/vault"
)

func pushSSH(t *testing.T, s *Service, o SSHPushOptions) map[string]SSHFileResult {
	t.Helper()
	res, err := s.SSHPush(o)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]SSHFileResult{}
	for _, f := range res.Files {
		out[f.Rel] = f
	}
	return out
}

func loadSSHState(t *testing.T, s *Service) sshdir.State {
	t.Helper()
	st, err := sshdir.LoadState(s.Paths.SSHState())
	if err != nil {
		t.Fatal(err)
	}
	return st
}

func vaultSSH(t *testing.T, s *Service, rel string) string {
	t.Helper()
	e, err := s.Get(SSHPrefix + rel)
	if err != nil {
		t.Fatal(err)
	}
	f, err := SSHFile(e)
	if err != nil {
		t.Fatal(err)
	}
	return string(f.Data)
}

func TestSSHPushEveryCase(t *testing.T) {
	t.Parallel()
	s, dir := sshService(t)
	plantSSHCases(t, s, dir)
	got := pushSSH(t, s, SSHPushOptions{})
	want := map[string]SSHAction{
		"same": SSHNoChange, "same_nobase": SSHNoChange, "local_new": SSHPushed, "remote_new": SSHNeedsPull,
		"local_changed": SSHPushed, "remote_change": SSHNeedsPull, "local_deleted": SSHKeptDelete,
		"remote_delete": SSHNeedsPull, "conflict": SSHKeptConflict, "conflict_new": SSHKeptConflict,
	}
	for name, a := range want {
		if f := got["config.d/"+name]; f.Action != a || f.Err != nil {
			t.Errorf("%s: got %+v, want %s", name, f, a)
		}
	}
	for rel, c := range sshCases {
		if rel == "config.d/gone" {
			continue
		}
		wantVault := c.remote
		if got[rel].Action == SSHPushed {
			wantVault = c.disk
		}
		if wantVault != "" && vaultSSH(t, s, rel) != wantVault {
			t.Errorf("%s: vault %q, want %q", rel, vaultSSH(t, s, rel), wantVault)
		}
	}
	checkPushedState(t, s)
}

func checkPushedState(t *testing.T, s *Service) {
	t.Helper()
	st := loadSSHState(t, s)
	want := map[string]string{
		"same": "Host a\n", "same_nobase": "Host a\n", "local_new": "Host a\n", "local_changed": "Host b\n",
		"remote_change": "Host a\n", "local_deleted": "Host a\n", "remote_delete": "Host a\n", "conflict": "Host a\n",
	}
	if len(st.Files) != len(want) || len(st.Skipped) != 0 {
		t.Fatalf("state %+v", st)
	}
	for name, base := range want {
		fs := st.Files["config.d/"+name]
		if fs.Sum != sha256.Sum256([]byte(base)) {
			t.Errorf("%s: wrong base", name)
		}
	}
	if fs := st.Files["config.d/local_new"]; fs.Size != int64(len("Host a\n")) || fs.ModTime.IsZero() {
		t.Errorf("local_new stat %+v", fs)
	}
}

func TestSSHPushOverwritesConflicts(t *testing.T) {
	t.Parallel()
	s, dir := sshService(t)
	plantSSHCases(t, s, dir)
	got := pushSSH(t, s, SSHPushOptions{Overwrite: true})
	for _, rel := range []string{"config.d/conflict", "config.d/conflict_new"} {
		if got[rel].Action != SSHPushed || vaultSSH(t, s, rel) != "Host b\n" {
			t.Errorf("%s: %+v", rel, got[rel])
		}
	}
}

func TestSSHPushAsksConflicts(t *testing.T) {
	t.Parallel()
	s, dir := sshService(t)
	plantSSHCases(t, s, dir)
	var asked []string
	ask := func(rel string) (bool, error) {
		asked = append(asked, rel)
		return rel == "config.d/conflict", nil
	}
	got := pushSSH(t, s, SSHPushOptions{AskConflict: ask})
	if !slices.Equal(asked, []string{"config.d/conflict", "config.d/conflict_new"}) {
		t.Fatalf("asked %v", asked)
	}
	if got["config.d/conflict"].Action != SSHPushed || got["config.d/conflict_new"].Action != SSHKeptConflict {
		t.Fatalf("got %+v", got)
	}
}

func plantSSHKeys(t *testing.T, dir string, names ...string) {
	t.Helper()
	priv, pub := sshKeyPair(t)
	writeSSHFile(t, dir, "config", []byte("IdentityFile ~/.ssh/used_key\n"))
	writeSSHFile(t, dir, "used_key", priv)
	writeSSHFile(t, dir, "used_key.pub", pub)
	for _, n := range names {
		writeSSHFile(t, dir, n, priv)
	}
}

func answer(t *testing.T, yes map[string]bool) func(string) (bool, error) {
	return func(rel string) (bool, error) {
		v, ok := yes[rel]
		if !ok {
			t.Errorf("asked about %s", rel)
		}
		delete(yes, rel)
		return v, nil
	}
}

func TestSSHPushKeyPicker(t *testing.T) {
	t.Parallel()
	s, dir := sshService(t)
	plantSSHKeys(t, dir, "work_key", "old_key")
	got := pushSSH(t, s, SSHPushOptions{AskKey: answer(t, map[string]bool{"work_key": true, "old_key": false})})
	for rel, a := range map[string]SSHAction{"config": SSHPushed, "used_key": SSHPushed, "used_key.pub": SSHPushed, "work_key": SSHPushed, "old_key": SSHSkippedKey} {
		if got[rel].Action != a {
			t.Errorf("%s: %+v, want %s", rel, got[rel], a)
		}
	}
	if st := loadSSHState(t, s); !slices.Equal(st.Skipped, []string{"old_key"}) {
		t.Fatalf("skipped %v", st.Skipped)
	}
	writeSSHFile(t, dir, "new_key", []byte(vaultSSH(t, s, "work_key")))
	got = pushSSH(t, s, SSHPushOptions{AskKey: answer(t, map[string]bool{"new_key": false})})
	if got["old_key"].Action != SSHSkippedKey || got["new_key"].Action != SSHSkippedKey {
		t.Fatalf("second push %+v", got)
	}
	if st := loadSSHState(t, s); !slices.Equal(st.Skipped, []string{"new_key", "old_key"}) {
		t.Fatalf("skipped %v", st.Skipped)
	}
}

func TestSSHPushKeysWithoutTerminal(t *testing.T) {
	t.Parallel()
	s, dir := sshService(t)
	plantSSHKeys(t, dir, "work_key")
	got := pushSSH(t, s, SSHPushOptions{})
	if got["work_key"].Action != SSHNotAsked || got["used_key"].Action != SSHPushed {
		t.Fatalf("got %+v", got)
	}
	if _, err := s.Get(SSHPrefix + "work_key"); !errors.Is(err, vault.ErrNotFound) {
		t.Fatalf("work_key in vault: %v", err)
	}
	if st := loadSSHState(t, s); len(st.Skipped) != 0 {
		t.Fatalf("skipped %v", st.Skipped)
	}
	if err := sshdir.SaveState(s.Paths.SSHState(), sshdir.State{Skipped: []string{"work_key"}}); err != nil {
		t.Fatal(err)
	}
	if got = pushSSH(t, s, SSHPushOptions{All: true}); got["work_key"].Action != SSHPushed {
		t.Fatalf("all: %+v", got["work_key"])
	}
	if st := loadSSHState(t, s); len(st.Skipped) != 0 || len(st.Files) != 4 {
		t.Fatalf("state %+v", st)
	}
}

func TestSSHPushReportsBadFiles(t *testing.T) {
	t.Parallel()
	s, dir := sshService(t)
	writeSSHFile(t, dir, "config", []byte("Host a\n"))
	writeSSHFile(t, dir, "config.d/binary", []byte{0xff, 0xfe, '\n'})
	writeSSHFile(t, dir, "config.d/taken", []byte("Host b\n"))
	if _, err := s.Add(vault.Entry{Path: SSHPrefix + "config.d/taken", Type: vault.TypeNote}); err != nil {
		t.Fatal(err)
	}
	got := pushSSH(t, s, SSHPushOptions{})
	if f := got["config.d/binary"]; f.Action != SSHFailed || !errors.Is(f.Err, ErrSSHFile) {
		t.Errorf("binary: %+v", f)
	}
	if f := got["config.d/taken"]; f.Action != SSHFailed || !errors.Is(f.Err, vault.ErrDuplicatePath) {
		t.Errorf("taken: %+v", f)
	}
	if got["config"].Action != SSHPushed || vaultSSH(t, s, "config") != "Host a\n" {
		t.Errorf("config: %+v", got["config"])
	}
	if st := loadSSHState(t, s); len(st.Files) != 1 {
		t.Errorf("state %+v", st)
	}
}

func TestSSHPushRefusesAVaultChangedSinceStatus(t *testing.T) {
	t.Parallel()
	s, _ := sshService(t)
	old := sshdir.File{Rel: "config", Kind: sshdir.KindConfig, Data: []byte("Host a\n")}
	addSSHEntry(t, s, sshdir.File{Rel: "config", Kind: sshdir.KindConfig, Data: []byte("Host c\n")})
	disk := sshdir.File{Rel: "config", Kind: sshdir.KindConfig, Data: []byte("Host b\n")}
	for _, want := range []*sshdir.File{nil, &old} {
		_, err := s.write(func(v *vault.Vault) (vault.Entry, error) {
			return vault.Entry{}, putSSH(v, SSHFileStatus{Rel: "config", Disk: &disk, Vault: want})
		})
		if !errors.Is(err, ErrStale) {
			t.Fatalf("vault %v: %v", want, err)
		}
	}
}

func TestSSHPushKeepsAConflictDeletedHere(t *testing.T) {
	t.Parallel()
	s, _ := sshService(t)
	addSSHEntry(t, s, sshdir.File{Rel: "config.d/x", Kind: sshdir.KindConfig, Data: []byte("Host c\n")})
	st := sshdir.State{Files: map[string]sshdir.FileState{"config.d/x": {Sum: sha256.Sum256([]byte("Host a\n"))}}}
	if err := sshdir.SaveState(s.Paths.SSHState(), st); err != nil {
		t.Fatal(err)
	}
	got := pushSSH(t, s, SSHPushOptions{Overwrite: true})
	if f := got["config.d/x"]; f.Change != SSHConflict || f.Action != SSHKeptConflict || vaultSSH(t, s, "config.d/x") != "Host c\n" {
		t.Fatalf("got %+v", f)
	}
}
