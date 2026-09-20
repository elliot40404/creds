package gitsync

import (
	"context"
	"errors"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/elliot40404/creds/internal/config"
	"github.com/elliot40404/creds/internal/vault"
)

func needSSHKeygen(t *testing.T) {
	t.Helper()
	needGit(t)
	if _, err := exec.LookPath("ssh-keygen"); err != nil {
		t.Skip("ssh-keygen not on PATH")
	}
}

func throwawayKey(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "id_ed25519")
	cmd := exec.Command("ssh-keygen")
	cmd.Args = append(cmd.Args, "-t", "ed25519", "-N", "", "-C", "creds-test", "-f", path)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Skipf("ssh-keygen failed: %v %s", err, out)
	}
	return path + ".pub"
}

func TestCommitIsSignedWithSSH(t *testing.T) {
	needSSHKeygen(t)
	r := newRepo(t)
	r.Configure(config.Sync{Sign: config.SignSSH, SignKey: throwawayKey(t)})
	writeVault(t, r)
	if !mustCommit(t, r, "init") {
		t.Fatal("nothing committed")
	}
	if body := gitOut(t, r, "cat-file", "commit", "HEAD"); !strings.Contains(body, "gpgsig") {
		t.Fatalf("commit carries no signature:\n%s", body)
	}
}

func TestCommitIsUnsignedByDefault(t *testing.T) {
	t.Parallel()
	needGit(t)
	r := newRepo(t)
	writeVault(t, r)
	if !mustCommit(t, r, "init") {
		t.Fatal("nothing committed")
	}
	if body := gitOut(t, r, "cat-file", "commit", "HEAD"); strings.Contains(body, "gpgsig") {
		t.Fatalf("default install signed a commit:\n%s", body)
	}
}

func TestCommitFailsLoudlyWhenSigningFails(t *testing.T) {
	t.Parallel()
	needGit(t)
	r := newRepo(t)
	r.Configure(config.Sync{Sign: config.SignSSH, SignKey: filepath.Join(t.TempDir(), "missing.pub")})
	writeVault(t, r)
	changed, err := r.Commit(context.Background(), "init")
	if err == nil {
		t.Fatal("a missing signing key committed anyway, unsigned")
	}
	if !errors.Is(err, ErrSignFailed) {
		t.Fatalf("err %v, want ErrSignFailed", err)
	}
	if changed {
		t.Fatal("reported a change after a failed commit")
	}
	if _, err := r.git.Run(context.Background(), "rev-parse", "--verify", "HEAD"); err == nil {
		t.Fatal("a commit landed despite the signing failure")
	}
}

func TestMergeCommitIsSigned(t *testing.T) {
	needSSHKeygen(t)
	key := throwawayKey(t)
	p := newVaultPair(t)
	p.a.Configure(config.Sync{Sign: config.SignSSH, SignKey: key})
	p.b.Configure(config.Sync{Sign: config.SignSSH, SignKey: key})
	x, y := secretEntry("x", "1"), secretEntry("y", "2")
	y.ID = idNearSlot(vault.SlotOf(x.ID), true)
	p.put(t, p.a, x)
	p.sync(t, p.a, Pushed)
	p.put(t, p.b, y)
	p.sync(t, p.b, Merged)
	merge := gitOut(t, p.b, "rev-list", "--merges", "-n", "1", "HEAD")
	if merge == "" {
		t.Fatal("no merge commit")
	}
	if body := gitOut(t, p.b, "cat-file", "commit", merge); !strings.Contains(body, "gpgsig") {
		t.Fatalf("merge commit is unsigned, commit-tree is missing -S:\n%s", body)
	}
}

func TestCommitIdentityDefaultsAndOverrides(t *testing.T) {
	t.Parallel()
	needGit(t)
	r := newRepo(t)
	writeVault(t, r)
	mustCommit(t, r, "one")
	want := r.git.userName() + " <" + r.git.userEmail() + ">"
	if got := gitOut(t, r, "log", "-1", "--format=%an <%ae>"); got != want {
		t.Fatalf("default identity %q, want %q", got, want)
	}
	r.Configure(config.Sync{Name: "Elliot", Email: "me@example.com"})
	writeFile(t, r, "entries/00.enc", ageBlob+"x\n")
	mustCommit(t, r, "two")
	if got := gitOut(t, r, "log", "-1", "--format=%an <%ae>"); got != "Elliot <me@example.com>" {
		t.Fatalf("custom identity %q", got)
	}
	r.Configure(config.Sync{})
	writeFile(t, r, "entries/00.enc", ageBlob+"y\n")
	mustCommit(t, r, "three")
	if got := gitOut(t, r, "log", "-1", "--format=%an <%ae>"); got != want {
		t.Fatalf("blank did not go back to %q: %q", want, got)
	}
}

func TestIdentityPrecedence(t *testing.T) {
	needGit(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("XDG_CONFIG_HOME", home)

	bare := NewGit(t.TempDir())
	if bare.userName() != UserName || bare.userEmail() != UserEmail {
		t.Fatalf("no global config should fall back to the fixed identity, got %s <%s>",
			bare.userName(), bare.userEmail())
	}

	g := NewGit(t.TempDir())
	if _, err := g.Run(context.Background(), "config", "--global", "user.name", "Global Person"); err != nil {
		t.Fatal(err)
	}
	if _, err := g.Run(context.Background(), "config", "--global", "user.email", "global@example.com"); err != nil {
		t.Fatal(err)
	}

	found := NewGit(t.TempDir())
	if found.userName() != "Global Person" || found.userEmail() != "global@example.com" {
		t.Fatalf("global identity not picked up, got %s <%s>", found.userName(), found.userEmail())
	}

	set := NewGit(t.TempDir())
	set.email = "explicit@example.com"
	if set.userEmail() != "explicit@example.com" {
		t.Fatalf("sync.email lost to the global one: %q", set.userEmail())
	}
	if set.userName() != "Global Person" {
		t.Fatalf("setting only the email changed the name: %q", set.userName())
	}
}

func TestGlobalIdentityIgnoresBrokenValues(t *testing.T) {
	needGit(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("XDG_CONFIG_HOME", home)
	g := NewGit(t.TempDir())
	if _, err := g.Run(context.Background(), "config", "--global", "user.email", "a<b>@example.com"); err != nil {
		t.Fatal(err)
	}
	if got := NewGit(t.TempDir()).userEmail(); got != UserEmail {
		t.Fatalf("angle brackets got through: %q", got)
	}
}
