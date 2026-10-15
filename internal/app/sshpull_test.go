package app

import (
	"crypto/sha256"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/elliot40404/creds/internal/fsutil"
	"github.com/elliot40404/creds/internal/sshdir"
)

func pullSSH(t *testing.T, s *Service, o SSHPullOptions) map[string]SSHFileResult {
	t.Helper()
	res, err := s.SSHPull(o)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]SSHFileResult{}
	for _, f := range res.Files {
		out[f.Rel] = f
	}
	return out
}

func diskSSH(t *testing.T, dir, rel string) string {
	t.Helper()
	data, err := fsutil.ReadFileLimit(filepath.Join(dir, filepath.FromSlash(rel)), sshdir.MaxFileSize)
	if errors.Is(err, fs.ErrNotExist) {
		return ""
	}
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func plantSkipped(t *testing.T, s *Service) {
	t.Helper()
	_, err := sshdir.UpdateState(s.Paths.SSHState(), func(st *sshdir.State) (bool, error) {
		st.Skipped = []string{"old_key"}
		return true, nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestSSHPullEveryCase(t *testing.T) {
	t.Parallel()
	s, dir := sshService(t)
	plantSSHCases(t, s, dir)
	plantSkipped(t, s)
	got := pullSSH(t, s, SSHPullOptions{})
	want := map[string]SSHAction{
		"same": SSHNoChange, "same_nobase": SSHNoChange, "local_new": SSHNeedsPush, "remote_new": SSHCreated,
		"local_changed": SSHNeedsPush, "remote_change": SSHUpdated, "local_deleted": SSHNeedsPush,
		"remote_delete": SSHNotAsked, "conflict": SSHKeptConflict, "conflict_new": SSHKeptConflict,
	}
	for name, a := range want {
		if f := got["config.d/"+name]; f.Action != a || f.Err != nil {
			t.Errorf("%s: got %+v, want %s", name, f, a)
		}
	}
	for rel, c := range sshCases {
		wantDisk := c.disk
		if a := got[rel].Action; a == SSHCreated || a == SSHUpdated {
			wantDisk = c.remote
		}
		if diskSSH(t, dir, rel) != wantDisk {
			t.Errorf("%s: disk %q, want %q", rel, diskSSH(t, dir, rel), wantDisk)
		}
	}
	checkPulledState(t, s)
}

func checkPulledState(t *testing.T, s *Service) {
	t.Helper()
	st := loadSSHState(t, s)
	want := map[string]string{
		"same": "Host a\n", "same_nobase": "Host a\n", "remote_new": "Host a\n", "local_changed": "Host a\n",
		"remote_change": "Host b\n", "local_deleted": "Host a\n", "remote_delete": "Host a\n", "conflict": "Host a\n",
	}
	if len(st.Files) != len(want) || len(st.Skipped) != 1 || st.Skipped[0] != "old_key" {
		t.Fatalf("state %+v", st)
	}
	for name, base := range want {
		if st.Files["config.d/"+name].Sum != sha256.Sum256([]byte(base)) {
			t.Errorf("%s: wrong base", name)
		}
	}
	if fs := st.Files["config.d/remote_new"]; fs.Size != int64(len("Host a\n")) || fs.ModTime.IsZero() {
		t.Errorf("remote_new stat %+v", fs)
	}
}

func TestSSHPullAsks(t *testing.T) {
	t.Parallel()
	s, dir := sshService(t)
	plantSSHCases(t, s, dir)
	yes := map[string]bool{"config.d/conflict": true, "config.d/conflict_new": false, "config.d/remote_delete": true}
	ask := answer(t, yes)
	byStatus := func(f SSHFileStatus) (bool, error) { return ask(f.Rel) }
	got := pullSSH(t, s, SSHPullOptions{AskConflict: byStatus, AskDelete: byStatus})
	if len(yes) != 0 {
		t.Fatalf("not asked %v", yes)
	}
	if got["config.d/conflict"].Action != SSHUpdated || got["config.d/conflict_new"].Action != SSHKeptConflict ||
		got["config.d/remote_delete"].Action != SSHDeleted {
		t.Fatalf("got %+v", got)
	}
	if diskSSH(t, dir, "config.d/conflict") != "Host c\n" || diskSSH(t, dir, "config.d/remote_delete") != "" {
		t.Fatal("disk not written")
	}
	st := loadSSHState(t, s)
	if _, ok := st.Files["config.d/remote_delete"]; ok || st.Files["config.d/conflict"].Sum != sha256.Sum256([]byte("Host c\n")) {
		t.Fatalf("state %+v", st)
	}
}

func TestSSHPullOverwriteNeverDeletes(t *testing.T) {
	t.Parallel()
	s, dir := sshService(t)
	plantSSHCases(t, s, dir)
	got := pullSSH(t, s, SSHPullOptions{Overwrite: true})
	for _, rel := range []string{"config.d/conflict", "config.d/conflict_new"} {
		if got[rel].Action != SSHUpdated || diskSSH(t, dir, rel) != "Host c\n" {
			t.Errorf("%s: %+v", rel, got[rel])
		}
	}
	if got["config.d/remote_delete"].Action != SSHNotAsked || diskSSH(t, dir, "config.d/remote_delete") != "Host a\n" {
		t.Fatalf("deleted %+v", got["config.d/remote_delete"])
	}
}

func TestSSHPullDeletedHereAndChangedInVault(t *testing.T) {
	t.Parallel()
	s, dir := sshService(t)
	addSSHEntry(t, s, sshdir.File{Rel: "config", Kind: sshdir.KindConfig, Data: []byte("Host b\n")})
	writeSSHFile(t, dir, "id_x.pub", []byte("ssh-ed25519 AAAA x\n"))
	st := sshdir.State{Files: map[string]sshdir.FileState{
		"config":   {Sum: sha256.Sum256([]byte("Host a\n"))},
		"id_x.pub": {Sum: sha256.Sum256([]byte("ssh-ed25519 AAAA old\n"))},
	}}
	if err := sshdir.SaveState(s.Paths.SSHState(), st); err != nil {
		t.Fatal(err)
	}
	var deletes []string
	got := pullSSH(t, s, SSHPullOptions{Overwrite: true, AskDelete: func(f SSHFileStatus) (bool, error) {
		deletes = append(deletes, f.Rel)
		return false, nil
	}})
	if got["config"].Action != SSHCreated || diskSSH(t, dir, "config") != "Host b\n" {
		t.Fatalf("config %+v", got["config"])
	}
	if got["id_x.pub"].Action != SSHKeptFile || len(deletes) != 1 || diskSSH(t, dir, "id_x.pub") == "" {
		t.Fatalf("pub %+v asked %v", got["id_x.pub"], deletes)
	}
}

func TestSSHPullCreatesTheSSHDir(t *testing.T) {
	t.Parallel()
	s, dir := sshService(t)
	addSSHEntry(t, s, sshdir.File{Rel: "config.d/work/hosts", Kind: sshdir.KindConfig, Data: []byte("Host w\n")})
	got := pullSSH(t, s, SSHPullOptions{})
	if got["config.d/work/hosts"].Action != SSHCreated || diskSSH(t, dir, "config.d/work/hosts") != "Host w\n" {
		t.Fatalf("got %+v", got)
	}
	checkSSHPerms(t, dir, "config.d/work/hosts")
}

func TestSSHPullRefusesAFileChangedSinceStatus(t *testing.T) {
	t.Parallel()
	s, dir := sshService(t)
	plantSSHCases(t, s, dir)
	got := pullSSH(t, s, SSHPullOptions{AskConflict: func(f SSHFileStatus) (bool, error) {
		writeSSHFile(t, dir, f.Rel, []byte("Host edited\n"))
		return true, nil
	}, AskDelete: func(SSHFileStatus) (bool, error) {
		writeSSHFile(t, dir, "config.d/remote_delete", []byte("Host edited\n"))
		return true, nil
	}})
	for _, rel := range []string{"config.d/conflict", "config.d/remote_delete"} {
		if f := got[rel]; f.Action != SSHFailed || !errors.Is(f.Err, ErrStale) || diskSSH(t, dir, rel) != "Host edited\n" {
			t.Errorf("%s: %+v", rel, f)
		}
	}
	if base := loadSSHState(t, s).Files["config.d/conflict"]; base.Sum != sha256.Sum256([]byte("Host a\n")) {
		t.Fatal("base moved on a failed write")
	}
}

func TestSSHPullRefusesLinks(t *testing.T) {
	t.Parallel()
	s, dir := sshService(t)
	outside := t.TempDir()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(dir, "config.d")); err != nil {
		t.Skipf("no symlinks: %v", err)
	}
	if err := os.Symlink(filepath.Join(outside, "target"), filepath.Join(dir, "config")); err != nil {
		t.Fatal(err)
	}
	addSSHEntry(t, s, sshdir.File{Rel: "config.d/a", Kind: sshdir.KindConfig, Data: []byte("Host a\n")})
	addSSHEntry(t, s, sshdir.File{Rel: "config", Kind: sshdir.KindConfig, Data: []byte("Host b\n")})
	got := pullSSH(t, s, SSHPullOptions{})
	for _, rel := range []string{"config.d/a", "config"} {
		if f := got[rel]; f.Action != SSHFailed || !errors.Is(f.Err, ErrSSHPath) {
			t.Errorf("%s: %+v", rel, f)
		}
	}
	entries, err := os.ReadDir(outside)
	if err != nil || len(entries) != 0 {
		t.Fatalf("wrote outside: %v %v", entries, err)
	}
}
