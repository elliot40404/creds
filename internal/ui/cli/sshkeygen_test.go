package cli

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/elliot40404/creds/internal/fail"
	"golang.org/x/crypto/ssh"
)

func keygenFile(t *testing.T, dir, rel string) []byte {
	t.Helper()
	path := filepath.Clean(filepath.Join(dir, rel))
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(path); runtime.GOOS != "windows" && (err != nil || info.Mode().Perm() != 0o600) {
		t.Fatalf("%s mode %v %v", rel, info, err)
	}
	return data
}

func TestSSHKeygenWritesKeyPair(t *testing.T) {
	t.Parallel()
	h, dir := sshHarness(t)
	r := h.ok(&fake{}, "ssh", "keygen", "id_work", "--comment", "me@laptop")
	if r.out != "created      id_work\ncreated      id_work.pub\n" ||
		!strings.Contains(r.err, "public key: ssh-ed25519 ") ||
		!strings.Contains(r.err, "creds ssh add-host <group/name> --host <host> --key id_work") {
		t.Fatalf("%q %q", r.out, r.err)
	}
	priv, pub := keygenFile(t, dir, "id_work"), keygenFile(t, dir, "id_work.pub")
	if _, err := ssh.ParseRawPrivateKey(priv); err != nil {
		t.Fatal(err)
	}
	if _, comment, _, _, err := ssh.ParseAuthorizedKey(pub); err != nil || comment != "me@laptop" {
		t.Fatalf("pub %q %v", pub, err)
	}
	if r = h.ok(&fake{}, "get", "ssh/id_work", "--field", "content", "--show"); !strings.HasPrefix(r.out, string(priv)) {
		t.Fatalf("vault %q", r.out)
	}
	if r = h.ok(&fake{}, "ssh", "status"); strings.Contains(r.out, "id_work") {
		t.Fatalf("status %q", r.out)
	}
	r = h.ok(&fake{}, "ssh", "keygen", "id_json", "--json")
	var got jsonSSHKeygen
	if err := json.Unmarshal([]byte(r.out), &got); err != nil {
		t.Fatalf("%v: %q", err, r.out)
	}
	if got.Path != "id_json" || got.Dir != dir || len(got.Files) != 2 || got.Files[1].Path != "id_json.pub" ||
		got.Files[0].Action != "created" || got.PublicKey != strings.TrimSpace(string(keygenFile(t, dir, "id_json.pub"))) {
		t.Fatalf("json %+v", got)
	}
}

func TestSSHKeygenPassphrase(t *testing.T) {
	t.Parallel()
	h, dir := sshHarness(t)
	f := &fake{passwords: []string{"open sesame", "open sesame"}}
	h.ok(f, "ssh", "keygen", "id_safe", "--passphrase")
	priv := keygenFile(t, dir, "id_safe")
	var missing *ssh.PassphraseMissingError
	if _, err := ssh.ParseRawPrivateKey(priv); !errors.As(err, &missing) {
		t.Fatalf("not encrypted: %v", err)
	}
	if _, err := ssh.ParseRawPrivateKeyWithPassphrase(priv, []byte("open sesame")); err != nil {
		t.Fatal(err)
	}
	r := h.fail(&fake{noTTY: true}, "ssh", "keygen", "id_tty", "--passphrase")
	if r.code != int(fail.Usage) || !strings.Contains(r.err, "drop --passphrase") {
		t.Fatalf("no tty: %d %q", r.code, r.err)
	}
	if _, err := os.Stat(filepath.Join(dir, "id_tty")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("written: %v", err)
	}
}

func TestSSHKeygenRefuses(t *testing.T) {
	t.Parallel()
	h, _ := sshHarness(t)
	for _, name := range []string{"a/b", "config", "id.pub", "known_hosts"} {
		if r := h.fail(&fake{}, "ssh", "keygen", name); r.code != int(fail.Usage) || !strings.Contains(r.err, "creds ssh keygen") {
			t.Errorf("%s: %d %q", name, r.code, r.err)
		}
	}
	if r := h.fail(&fake{}, "ssh", "keygen", "used_key"); r.code != int(fail.Conflict) {
		t.Fatalf("existing: %d %q", r.code, r.err)
	}
}

func TestSSHKeygenInteractive(t *testing.T) {
	t.Parallel()
	h, dir := sshHarness(t)
	f := &fake{inputs: []string{"id_asked"}, confirms: []bool{true}, passwords: []string{"pw", "pw"}}
	r := h.ok(f, "ssh", "keygen", "-i")
	if !strings.Contains(r.err, "command: creds ssh keygen id_asked --passphrase") {
		t.Fatalf("%q %q", f.prompts, r.err)
	}
	if _, err := ssh.ParseRawPrivateKeyWithPassphrase(keygenFile(t, dir, "id_asked"), []byte("pw")); err != nil {
		t.Fatal(err)
	}
}
