package app

import (
	"crypto/sha256"
	"errors"
	"testing"

	"github.com/elliot40404/creds/internal/sshdir"
)

func sshHostService(t *testing.T, config string) (*Service, string) {
	t.Helper()
	s, dir := sshService(t)
	priv, pub := sshKeyPair(t)
	writeSSHFile(t, dir, "id_ed25519", priv)
	writeSSHFile(t, dir, "id_ed25519.pub", pub)
	if config != "" {
		writeSSHFile(t, dir, "config", []byte(config))
	}
	return s, dir
}

func TestSSHAddHostWritesDiskVaultAndState(t *testing.T) {
	t.Parallel()
	s, dir := sshHostService(t, "Include config.d/*/*\n")
	res, err := s.SSHAddHost(SSHHost{Name: "beam/data_new", Host: "10.0.0.5", User: "ubuntu", Port: 2222, Key: "id_ed25519"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Rel != "config.d/beam/data_new" || res.Alias != "data_new" || res.Include != "" || res.Dir != dir {
		t.Fatalf("result %+v", res)
	}
	want := "Host data_new\n  HostName 10.0.0.5\n  User ubuntu\n  Port 2222\n  IdentityFile ~/.ssh/id_ed25519\n  IdentitiesOnly yes\n"
	if got := diskSSH(t, dir, res.Rel); got != want {
		t.Fatalf("disk %q", got)
	}
	if got := vaultSSH(t, s, res.Rel); got != want {
		t.Fatalf("vault %q", got)
	}
	if got := loadSSHState(t, s).Files[res.Rel].Sum; got != sha256.Sum256([]byte(want)) {
		t.Fatalf("state sum %x", got)
	}
	st, err := s.SSHStatus()
	if err != nil {
		t.Fatal(err)
	}
	if f := byRel(st)[res.Rel]; f.Change != SSHSame {
		t.Fatalf("status %+v", f)
	}
}

func TestSSHAddHostIncludeHint(t *testing.T) {
	t.Parallel()
	cases := []struct{ config, name, want string }{
		{"", "plain", "Include config.d/*"},
		{"Host x\n", "beam/db", "Include config.d/*/*"},
		{"Include config.d/*\n", "beam/db", "Include config.d/*/*"},
		{"Include config.d/*\n", "plain", ""},
	}
	for _, tc := range cases {
		s, dir := sshHostService(t, tc.config)
		res, err := s.SSHAddHost(SSHHost{Name: tc.name, Host: "db.example.com", Port: 22})
		if err != nil {
			t.Fatal(err)
		}
		if res.Include != tc.want {
			t.Errorf("%q %s: include %q, want %q", tc.config, tc.name, res.Include, tc.want)
		}
		if got := diskSSH(t, dir, res.Rel); got != "Host "+res.Alias+"\n  HostName db.example.com\n" {
			t.Errorf("disk %q", got)
		}
	}
}

func TestSSHAddHostRefusesExisting(t *testing.T) {
	t.Parallel()
	s, dir := sshHostService(t, "")
	writeSSHFile(t, dir, "config.d/beam/here", []byte("Host here\n"))
	addSSHEntry(t, s, sshdir.File{Rel: "config.d/beam/vault", Kind: sshdir.KindConfig, Data: []byte("Host vault\n")})
	for _, name := range []string{"beam/here", "beam/vault"} {
		if _, err := s.SSHAddHost(SSHHost{Name: name, Host: "h"}); !errors.Is(err, ErrSSHExists) {
			t.Errorf("%s: %v", name, err)
		}
	}
	if got := diskSSH(t, dir, "config.d/beam/here"); got != "Host here\n" {
		t.Fatalf("overwritten %q", got)
	}
	if got := diskSSH(t, dir, "config.d/beam/vault"); got != "" {
		t.Fatalf("vault only file written %q", got)
	}
}

func TestSSHAddHostRefusesBadInput(t *testing.T) {
	t.Parallel()
	s, dir := sshHostService(t, "")
	bad := []SSHHost{
		{Name: "", Host: "h"},
		{Name: "a/b/c", Host: "h"},
		{Name: "../x", Host: "h"},
		{Name: "a b", Host: "h"},
		{Name: "web*", Host: "h"},
		{Name: "a/.hidden", Host: "h"},
		{Name: "x%h", Host: "h"},
		{Name: "x"},
		{Name: "x", Host: "a b"},
		{Name: "x", Host: "h\x1b"},
		{Name: "x", Host: "h", User: `"root"`},
		{Name: "x", Host: "h", Port: 65536},
		{Name: "x", Host: "h", Port: -1},
		{Name: "x", Host: "h", Key: "missing"},
		{Name: "x", Host: "h", Key: "id_ed25519.pub"},
		{Name: "x", Host: "h", Key: "../id_ed25519"},
	}
	for _, h := range bad {
		if _, err := s.SSHAddHost(h); !errors.Is(err, ErrSSHHost) {
			t.Errorf("%+v: %v", h, err)
		}
	}
	if got := diskSSH(t, dir, "config.d/x"); got != "" {
		t.Fatalf("written %q", got)
	}
}
