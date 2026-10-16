package app

import (
	"crypto/sha256"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/elliot40404/creds/internal/sshdir"
)

func sshActions(files []SSHFileResult) []string {
	var out []string
	for _, f := range files {
		out = append(out, f.Rel+" "+string(f.Action))
	}
	return out
}

func TestSSHAddKeyCopiesOutsideKey(t *testing.T) {
	t.Parallel()
	s, dir := sshService(t)
	priv, pub := sshKeyPair(t)
	out := t.TempDir()
	writeSSHFile(t, out, "work.pem", priv)
	writeSSHFile(t, out, "work.pem.pub", pub)
	res, err := s.SSHAddKey(filepath.Join(out, "work.pem"), "work_key")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"work_key created", "work_key.pub created"}
	if !slices.Equal(sshActions(res.Files), want) || res.Rel != "work_key" || !res.Unused || res.Dir != dir || res.PubNote != "" {
		t.Fatalf("result %+v", res)
	}
	for rel, data := range map[string][]byte{"work_key": priv, "work_key.pub": pub} {
		if diskSSH(t, dir, rel) != string(data) || vaultSSH(t, s, rel) != string(data) {
			t.Fatalf("%s not on disk and in the vault", rel)
		}
		if loadSSHState(t, s).Files[rel].Sum != sha256.Sum256(data) {
			t.Fatalf("%s state", rel)
		}
	}
	st, err := s.SSHStatus()
	if err != nil {
		t.Fatal(err)
	}
	if f := byRel(st)["work_key"]; f.Change != SSHSame {
		t.Fatalf("status %+v", f)
	}
	if res, err = s.SSHAddKey(filepath.Join(out, "work.pem"), "work_key"); err != nil {
		t.Fatal(err)
	}
	if got := sshActions(res.Files); !slices.Equal(got, []string{"work_key same", "work_key.pub same"}) {
		t.Fatalf("again %q", got)
	}
}

func TestSSHAddKeyDefaultNameAndBadPub(t *testing.T) {
	t.Parallel()
	s, dir := sshService(t)
	priv, _ := sshKeyPair(t)
	out := t.TempDir()
	writeSSHFile(t, out, "id_ed25519", priv)
	writeSSHFile(t, out, "id_ed25519.pub", []byte("not a key\n"))
	res, err := s.SSHAddKey(filepath.Join(out, "id_ed25519"), "")
	if err != nil {
		t.Fatal(err)
	}
	if got := sshActions(res.Files); !slices.Equal(got, []string{"id_ed25519 created"}) || res.Unused || !strings.Contains(res.PubNote, "not a public key") {
		t.Fatalf("result %q %+v", got, res)
	}
	if diskSSH(t, dir, "id_ed25519.pub") != "" {
		t.Fatal("bad pub copied")
	}
}

func TestSSHAddKeyPushesSkippedKeyInside(t *testing.T) {
	t.Parallel()
	s, dir := sshService(t)
	priv, pub := sshKeyPair(t)
	writeSSHFile(t, dir, "old_key", priv)
	writeSSHFile(t, dir, "old_key.pub", pub)
	if err := sshdir.SaveState(s.Paths.SSHState(), sshdir.State{Skipped: []string{"old_key", "other"}}); err != nil {
		t.Fatal(err)
	}
	res, err := s.SSHAddKey(filepath.Join(dir, "old_key"), "")
	if err != nil {
		t.Fatal(err)
	}
	if got := sshActions(res.Files); !slices.Equal(got, []string{"old_key pushed", "old_key.pub pushed"}) || !res.Unused {
		t.Fatalf("result %q %+v", got, res)
	}
	if vaultSSH(t, s, "old_key") != string(priv) || vaultSSH(t, s, "old_key.pub") != string(pub) {
		t.Fatal("vault")
	}
	if st := loadSSHState(t, s); !slices.Equal(st.Skipped, []string{"other"}) || st.Files["old_key"].Sum != sha256.Sum256(priv) {
		t.Fatalf("state %+v", st)
	}
}

func TestSSHAddKeyRefusesNonKeys(t *testing.T) {
	t.Parallel()
	s, dir := sshService(t)
	priv, pub := sshKeyPair(t)
	out := t.TempDir()
	files := map[string][]byte{
		"id.pub": pub, "config": []byte("Host a\n"), "notes.txt": []byte("hello\n"),
		"big": []byte(strings.Repeat("a", sshdir.MaxFileSize+1)), "bad name": priv, "key": priv,
	}
	for name, data := range files {
		writeSSHFile(t, out, name, data)
		writeSSHFile(t, dir, name, data)
	}
	writeSSHFile(t, dir, "known_hosts", priv)
	cases := [][2]string{
		{filepath.Join(out, "id.pub"), ""},
		{filepath.Join(out, "config"), "x"},
		{filepath.Join(out, "notes.txt"), ""},
		{filepath.Join(out, "big"), ""},
		{filepath.Join(out, "missing"), ""},
		{out, "x"},
		{filepath.Join(out, "bad name"), ""},
		{filepath.Join(out, "key"), "a/b"},
		{filepath.Join(out, "key"), "x.pub"},
		{filepath.Join(out, "key"), "config"},
		{filepath.Join(dir, "id.pub"), ""},
		{filepath.Join(dir, "config"), ""},
		{filepath.Join(dir, "known_hosts"), ""},
		{filepath.Join(dir, "big"), ""},
		{filepath.Join(dir, "key"), "other"},
	}
	for _, c := range cases {
		if _, err := s.SSHAddKey(c[0], c[1]); !errors.Is(err, ErrSSHKey) {
			t.Errorf("%s %q: %v", c[0], c[1], err)
		}
	}
	if diskSSH(t, dir, "x") != "" || diskSSH(t, dir, "a") != "" {
		t.Fatal("written")
	}
	if err := os.Symlink(filepath.Join(out, "key"), filepath.Join(out, "link")); err != nil {
		t.Skipf("no symlinks: %v", err)
	}
	if _, err := s.SSHAddKey(filepath.Join(out, "link"), ""); !errors.Is(err, ErrSSHKey) {
		t.Fatalf("link: %v", err)
	}
}

func TestSSHAddKeyRefusesOtherContent(t *testing.T) {
	t.Parallel()
	s, dir := sshService(t)
	priv, _ := sshKeyPair(t)
	other, _ := sshKeyPair(t)
	out := t.TempDir()
	writeSSHFile(t, out, "here", priv)
	writeSSHFile(t, out, "vault", priv)
	writeSSHFile(t, dir, "here", other)
	writeSSHFile(t, dir, "inside", priv)
	for _, rel := range []string{"vault", "inside"} {
		addSSHEntry(t, s, sshdir.File{Rel: rel, Kind: sshdir.KindPrivateKey, Data: other})
	}
	for _, p := range []string{filepath.Join(out, "here"), filepath.Join(out, "vault"), filepath.Join(dir, "inside")} {
		if _, err := s.SSHAddKey(p, ""); !errors.Is(err, ErrSSHExists) {
			t.Errorf("%s: %v", p, err)
		}
	}
	if diskSSH(t, dir, "here") != string(other) || diskSSH(t, dir, "vault") != "" || vaultSSH(t, s, "inside") != string(other) {
		t.Fatal("changed")
	}
}
