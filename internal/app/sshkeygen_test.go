package app

import (
	"crypto/sha256"
	"errors"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/elliot40404/creds/internal/sshdir"
	"golang.org/x/crypto/ssh"
)

func TestSSHKeygenWritesKeyPair(t *testing.T) {
	t.Parallel()
	s, fp, _, _ := initVault(t)
	fp.prompts, fp.warned = nil, nil
	dir := filepath.Join(s.UserHome, ".ssh")
	res, err := s.SSHKeygen(SSHKeygen{Name: "id_work"})
	if err != nil {
		t.Fatal(err)
	}
	if got := sshActions(res.Files); !slices.Equal(got, []string{"id_work created", "id_work.pub created"}) || res.Dir != dir || res.Rel != "id_work" {
		t.Fatalf("result %+v", res)
	}
	if len(fp.prompts) != 0 {
		t.Fatalf("asked %q", fp.prompts)
	}
	priv, pub := diskSSH(t, dir, "id_work"), diskSSH(t, dir, "id_work.pub")
	if _, err := ssh.ParseRawPrivateKey([]byte(priv)); err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(pub) != res.PublicKey || !strings.HasSuffix(res.PublicKey, " id_work") {
		t.Fatalf("pub %q %q", pub, res.PublicKey)
	}
	for rel, data := range map[string]string{"id_work": priv, "id_work.pub": pub} {
		if vaultSSH(t, s, rel) != data || loadSSHState(t, s).Files[rel].Sum != sha256.Sum256([]byte(data)) {
			t.Fatalf("%s not in the vault and state", rel)
		}
	}
}

func TestSSHKeygenWithPassphrase(t *testing.T) {
	t.Parallel()
	s, fp, _, _ := initVault(t)
	fp.prompts, fp.warned = nil, nil
	fp.passwords = []string{"", "one", "two", "open sesame", "open sesame"}
	res, err := s.SSHKeygen(SSHKeygen{Name: "id_safe", Comment: "me@laptop", Passphrase: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(fp.warned) != 2 || len(fp.passwords) != 0 || !strings.HasSuffix(res.PublicKey, " me@laptop") {
		t.Fatalf("warned %q left %q pub %q", fp.warned, fp.passwords, res.PublicKey)
	}
	priv := []byte(diskSSH(t, filepath.Join(s.UserHome, ".ssh"), "id_safe"))
	var missing *ssh.PassphraseMissingError
	if _, err := ssh.ParseRawPrivateKey(priv); !errors.As(err, &missing) {
		t.Fatalf("not encrypted: %v", err)
	}
	if _, err := ssh.ParseRawPrivateKeyWithPassphrase(priv, []byte("open sesame")); err != nil {
		t.Fatal(err)
	}
}

func TestSSHKeygenWithGivenPassphrase(t *testing.T) {
	t.Parallel()
	s, fp, _, _ := initVault(t)
	fp.prompts, fp.warned = nil, nil
	pass := []byte("open sesame")
	if _, err := s.SSHKeygen(SSHKeygen{Name: "id_given", Passphrase: true, PassphraseBytes: pass}); err != nil {
		t.Fatal(err)
	}
	if len(fp.prompts) != 0 || string(pass) != strings.Repeat("\x00", len(pass)) {
		t.Fatalf("asked %q, bytes %q", fp.prompts, pass)
	}
	priv := []byte(diskSSH(t, filepath.Join(s.UserHome, ".ssh"), "id_given"))
	if _, err := ssh.ParseRawPrivateKeyWithPassphrase(priv, []byte("open sesame")); err != nil {
		t.Fatal(err)
	}
}

func TestSSHKeygenRefusesEmptyPassphrase(t *testing.T) {
	t.Parallel()
	s, fp, _, _ := initVault(t)
	fp.prompts, fp.warned = nil, nil
	fp.passwords = []string{"", "", ""}
	if _, err := s.SSHKeygen(SSHKeygen{Name: "id_x", Passphrase: true}); !errors.Is(err, ErrSSHPassphrase) {
		t.Fatal(err)
	}
	if got := diskSSH(t, filepath.Join(s.UserHome, ".ssh"), "id_x"); got != "" {
		t.Fatalf("written %q", got)
	}
}

func TestSSHKeygenRefusesExisting(t *testing.T) {
	t.Parallel()
	s, dir := sshService(t)
	writeSSHFile(t, dir, "here.pub", []byte("x\n"))
	priv, _ := sshKeyPair(t)
	addSSHEntry(t, s, sshdir.File{Rel: "vault", Kind: sshdir.KindPrivateKey, Data: priv})
	for _, name := range []string{"here", "vault"} {
		if _, err := s.SSHKeygen(SSHKeygen{Name: name}); !errors.Is(err, ErrSSHExists) {
			t.Errorf("%s: %v", name, err)
		}
	}
	if diskSSH(t, dir, "here") != "" || diskSSH(t, dir, "vault") != "" {
		t.Fatal("written")
	}
}

func TestSSHKeygenRefusesBadInput(t *testing.T) {
	t.Parallel()
	s, _ := sshService(t)
	for _, k := range []SSHKeygen{
		{Name: ""},
		{Name: "a/b"},
		{Name: "../x"},
		{Name: "config"},
		{Name: "x.pub"},
		{Name: "known_hosts2"},
		{Name: ".hidden"},
		{Name: "x", Comment: "a\nb"},
		{Name: "x", Comment: strings.Repeat("a", 256)},
	} {
		if _, err := s.SSHKeygen(k); !errors.Is(err, ErrSSHKeygen) {
			t.Errorf("%+v: %v", k, err)
		}
	}
}
