package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSSHAutoPullAcrossDevices(t *testing.T) {
	t.Parallel()
	a, b := joined(t)
	adir, bdir := filepath.Join(a.user, ".ssh"), filepath.Join(b.user, ".ssh")
	if err := os.Mkdir(adir, 0o700); err != nil {
		t.Fatal(err)
	}
	writeFile(t, adir, "config", "Host a\n")
	a.ok(&fake{noTTY: true}, "ssh", "push")
	a.ok(&fake{}, "sync")
	b.ok(&fake{}, "sync")
	b.ok(&fake{noTTY: true}, "ssh", "pull")

	key := sshKeyPEM(t)
	writeFile(t, adir, "used_key", key)
	writeFile(t, adir, "config", "Host a\n  IdentityFile ~/.ssh/used_key\n")
	a.ok(&fake{noTTY: true}, "ssh", "push")
	a.ok(&fake{}, "sync")
	if r := b.ok(&fake{}, "sync"); strings.Contains(r.err, "ssh:") {
		t.Fatalf("hint on a clean pull: %q", r.err)
	}
	if got := readSSH(t, bdir, "config"); got != "Host a\n  IdentityFile ~/.ssh/used_key\n" {
		t.Fatalf("b config %q", got)
	}
	if got := readSSH(t, bdir, "used_key"); got != key {
		t.Fatal("b key not written")
	}

	writeFile(t, adir, "config", "Host a\n  Port 1\n")
	a.ok(&fake{}, "list")
	a.ok(&fake{}, "sync")
	writeFile(t, bdir, "config", "Host a\n  Port 2\n")
	r := b.ok(&fake{}, "sync")
	if !strings.Contains(r.err, "ssh: config changed here and in the vault, run creds ssh pull") {
		t.Fatalf("no conflict hint: %q", r.err)
	}
	if got := readSSH(t, bdir, "config"); got != "Host a\n  Port 2\n" {
		t.Fatalf("b edit overwritten: %q", got)
	}
	if r = b.ok(&fake{}, "list"); strings.Contains(r.err, "run creds ssh pull") {
		t.Fatalf("hint shown twice: %q", r.err)
	}
}
