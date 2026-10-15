package app

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"golang.org/x/crypto/ssh"

	"github.com/elliot40404/creds/internal/sshdir"
	"github.com/elliot40404/creds/internal/vault"
)

func TestSSHPathRoundTrip(t *testing.T) {
	t.Parallel()
	for _, rel := range []string{"config", "config.d/beam/data_new", "id_rsa.pem", ".hidden/key", "work key.ppk"} {
		p, err := SSHVaultPath(rel)
		if err != nil || p != "ssh/"+rel {
			t.Fatalf("SSHVaultPath(%q) = %q, %v", rel, p, err)
		}
		back, err := SSHRel(p)
		if err != nil || back != rel {
			t.Fatalf("SSHRel(%q) = %q, %v", p, back, err)
		}
	}
}

func TestSSHPathRefusedBothWays(t *testing.T) {
	t.Parallel()
	bad := []string{
		"", "..", "../x", "a/../b", "a/..", ".", "./a", "a/./b", "/etc/passwd", "/",
		"C:/Windows/x", "c:x", `a\b`, `\\server\share`, `..\x`, "a//b", "a/", " a", "a /b",
		"a\x00b", "a\nb", "id_rsa:stream",
	}
	for _, rel := range bad {
		if p, err := SSHVaultPath(rel); !errors.Is(err, ErrSSHPath) {
			t.Errorf("SSHVaultPath(%q) = %q, %v", rel, p, err)
		}
		if r, err := SSHRel(SSHPrefix + rel); !errors.Is(err, ErrSSHPath) {
			t.Errorf("SSHRel(%q) = %q, %v", SSHPrefix+rel, r, err)
		}
	}
}

func TestSSHRelNeedsPrefix(t *testing.T) {
	t.Parallel()
	for _, p := range []string{"ssh", "ssh/", "config", "sshx/config", "SSH/config", "/ssh/config", "work/ssh/config"} {
		if r, err := SSHRel(p); !errors.Is(err, ErrSSHPath) {
			t.Errorf("SSHRel(%q) = %q, %v", p, r, err)
		}
	}
}

func TestSSHEntryRoundTrip(t *testing.T) {
	t.Parallel()
	res, err := sshdir.Scan(sshTestDir(t))
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Files) != 4 {
		t.Fatalf("scanned %+v", res)
	}
	for _, f := range res.Files {
		e, err := SSHEntry(f)
		if err != nil {
			t.Fatalf("%s: %v", f.Rel, err)
		}
		checkSSHEntry(t, f, e)
		back, err := SSHFile(e)
		if err != nil {
			t.Fatalf("%s: %v", f.Rel, err)
		}
		if back.Rel != f.Rel || back.Kind != f.Kind || back.Sum() != f.Sum() {
			t.Fatalf("round trip %s: got %+v", f.Rel, back)
		}
	}
}

func checkSSHEntry(t *testing.T, f sshdir.File, e vault.Entry) {
	t.Helper()
	if err := e.Validate(); err != nil {
		t.Fatalf("%s: %v", f.Rel, err)
	}
	want := []vault.Field{{Name: FieldContent, Value: string(f.Data), Secret: f.Kind == sshdir.KindPrivateKey}}
	if e.Type != vault.TypeSSHFile || e.Path != "ssh/"+f.Rel || !slices.Equal(e.Fields, want) {
		t.Fatalf("entry %+v", e)
	}
}

func TestSSHEntryRefused(t *testing.T) {
	t.Parallel()
	for _, f := range []sshdir.File{
		{Rel: "../config", Kind: sshdir.KindConfig, Data: []byte("Host x\n")},
		{Rel: "config", Kind: sshdir.KindConfig, Data: []byte{'H', 0xff, 0xfe}},
	} {
		if e, err := SSHEntry(f); err == nil {
			t.Errorf("%q: got %+v", f.Rel, e)
		}
	}
}

func TestSSHFileRefused(t *testing.T) {
	t.Parallel()
	entry := func(path, content string) vault.Entry {
		return vault.Entry{Path: path, Type: vault.TypeSSHFile, Fields: []vault.Field{{Name: FieldContent, Value: content}}}
	}
	cases := map[string]struct {
		e    vault.Entry
		want error
	}{
		"dotdot":          {entry("ssh/../.bashrc", "x"), ErrSSHPath},
		"outside":         {entry("work/config", "x"), ErrSSHPath},
		"backslash":       {entry(`ssh/..\x`, "x"), ErrSSHPath},
		"drive":           {entry("ssh/C:/x", "x"), ErrSSHPath},
		"wrong type":      {vault.Entry{Path: "ssh/config", Type: vault.TypeNote}, ErrSSHFile},
		"no content":      {vault.Entry{Path: "ssh/config", Type: vault.TypeSSHFile}, ErrSSHFile},
		"too big":         {entry("ssh/config", strings.Repeat("a", sshdir.MaxFileSize+1)), ErrSSHFile},
		"authorized keys": {entry("ssh/authorized_keys.pub", "ssh-ed25519 AAAA"), ErrSSHFile},
		"known hosts":     {entry("ssh/config.d/known_hosts", "x"), ErrSSHFile},
		"other file":      {entry("ssh/tunnel", "x"), ErrSSHFile},
	}
	for name, c := range cases {
		if f, err := SSHFile(c.e); !errors.Is(err, c.want) {
			t.Errorf("%s: got %+v, %v", name, f, err)
		}
	}
}

func sshTestDir(t *testing.T) string {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	block, err := ssh.MarshalPrivateKey(priv, "")
	if err != nil {
		t.Fatal(err)
	}
	sp, err := ssh.NewPublicKey(pub)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	writeSSHFile(t, dir, "config", []byte("Include config.d/*/*\n"))
	writeSSHFile(t, dir, "config.d/beam/data_new", []byte("Host beam\n  IdentityFile ~/.ssh/id_ed25519\n"))
	writeSSHFile(t, dir, "id_ed25519", pem.EncodeToMemory(block))
	writeSSHFile(t, dir, "id_ed25519.pub", ssh.MarshalAuthorizedKey(sp))
	writeSSHFile(t, dir, "known_hosts", []byte("beam ssh-ed25519 AAAA\n"))
	return dir
}

func writeSSHFile(t *testing.T, dir, rel string, data []byte) {
	t.Helper()
	p := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, data, 0o600); err != nil {
		t.Fatal(err)
	}
}
