package sshdir

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/elliot40404/creds/internal/fsutil"
)

func writeTree(t *testing.T, files map[string][]byte) string {
	t.Helper()
	dir := t.TempDir()
	writeFiles(t, dir, files)
	return dir
}

func writeFiles(t *testing.T, dir string, files map[string][]byte) {
	t.Helper()
	for rel, data := range files {
		path := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

func sshTree(t *testing.T) map[string][]byte {
	t.Helper()
	ed := ed25519Key(t)
	rk := rsaKey(t)
	ek := ecdsaKey(t)
	edData := openssh(t, ed)
	return map[string][]byte{
		"id_ed25519":             edData,
		"id_ed25519.pub":         publicKey(t, ed),
		"id_ed25519_copy":        edData,
		"id_rsa":                 openssh(t, rk),
		"id_ecdsa":               openssh(t, ek),
		"locked":                 opensshLocked(t, ed),
		"legacy.pem":             legacyLocked(),
		"id_rsa.pem":             pkcs1(rk),
		"ecdsa.p8":               pkcs8(t, ek),
		"work.ppk":               []byte(putty),
		"id_ed25519_sk":          unknownOpenSSH(),
		"config":                 []byte("Include config.d/*\n"),
		"config.d/work":          []byte("Host work\n"),
		"config.d/beam/hosts":    []byte("Host beam\n"),
		"config.d/beam/data_new": openssh(t, ek),
		"known_hosts":            []byte("host ssh-ed25519 AAAA\n"),
		"known_hosts.old":        []byte("host ssh-ed25519 AAAA\n"),
		"authorized_keys":        []byte("ssh-ed25519 AAAA\n"),
		"tunnel":                 []byte("not a key\n"),
		"cert.pem":               []byte("-----BEGIN CERTIFICATE-----\nAAAA\n-----END CERTIFICATE-----\n"),
		"big.pub":                bytes.Repeat([]byte("a"), MaxFileSize+1),
		"edge.pub":               bytes.Repeat([]byte("a"), MaxFileSize),
	}
}

func TestScanKinds(t *testing.T) {
	res, err := Scan(writeTree(t, sshTree(t)))
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]Kind{
		"id_ed25519":             KindPrivateKey,
		"id_ed25519.pub":         KindPublicKey,
		"id_ed25519_copy":        KindPrivateKey,
		"id_rsa":                 KindPrivateKey,
		"id_ecdsa":               KindPrivateKey,
		"locked":                 KindPrivateKey,
		"legacy.pem":             KindPrivateKey,
		"id_rsa.pem":             KindPrivateKey,
		"ecdsa.p8":               KindPrivateKey,
		"work.ppk":               KindPrivateKey,
		"id_ed25519_sk":          KindPrivateKey,
		"config":                 KindConfig,
		"config.d/work":          KindConfig,
		"config.d/beam/hosts":    KindConfig,
		"config.d/beam/data_new": KindPrivateKey,
		"edge.pub":               KindPublicKey,
	}
	got := map[string]Kind{}
	for _, f := range res.Files {
		got[f.Rel] = f.Kind
	}
	for rel, kind := range want {
		if got[rel] != kind {
			t.Errorf("%s: kind %d, want %d", rel, got[rel], kind)
		}
	}
	if len(got) != len(want) {
		t.Errorf("got %d files, want %d: %v", len(got), len(want), got)
	}
}

func TestScanRefusesBigFile(t *testing.T) {
	res, err := Scan(writeTree(t, sshTree(t)))
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Skipped) != 1 || res.Skipped[0].Rel != "big.pub" || !errors.Is(res.Skipped[0].Err, fsutil.ErrTooLarge) {
		t.Fatalf("skipped = %v", res.Skipped)
	}
}

func TestScanKeepsContent(t *testing.T) {
	files := sshTree(t)
	res, err := Scan(writeTree(t, files))
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range res.Files {
		if !bytes.Equal(f.Data, files[f.Rel]) {
			t.Errorf("%s: content changed", f.Rel)
		}
		if f.Sum() != sha256.Sum256(files[f.Rel]) {
			t.Errorf("%s: wrong sum", f.Rel)
		}
	}
}

func TestScanKeys(t *testing.T) {
	res, err := Scan(writeTree(t, sshTree(t)))
	if err != nil {
		t.Fatal(err)
	}
	keys := res.Keys()
	if len(keys) != 11 {
		t.Fatalf("keys = %d, want 11", len(keys))
	}
	for _, k := range keys {
		if k.Kind != KindPrivateKey {
			t.Errorf("%s: not a key", k.Rel)
		}
	}
}

func TestScanSkipsSymlinks(t *testing.T) {
	dir := writeTree(t, map[string][]byte{"target/config": []byte("Host x\n")})
	if err := os.Symlink(filepath.Join(dir, "target", "config"), filepath.Join(dir, "config")); err != nil {
		t.Skipf("symlink: %v", err)
	}
	if err := os.Symlink(filepath.Join(dir, "target"), filepath.Join(dir, "config.d")); err != nil {
		t.Skipf("symlink: %v", err)
	}
	res, err := Scan(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Files) != 0 {
		t.Errorf("files = %v", res.Files)
	}
	if len(res.Skipped) != 2 {
		t.Fatalf("skipped = %v", res.Skipped)
	}
	for _, s := range res.Skipped {
		if !errors.Is(s.Err, ErrSymlink) {
			t.Errorf("%s: %v", s.Rel, s.Err)
		}
	}
}

func TestScanSlashPaths(t *testing.T) {
	res, err := Scan(writeTree(t, map[string][]byte{"config.d/a/b/c": []byte("Host c\n")}))
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Files) != 1 || res.Files[0].Rel != "config.d/a/b/c" || strings.Contains(res.Files[0].Rel, `\`) {
		t.Fatalf("files = %v", res.Files)
	}
}

func TestScanMissingDir(t *testing.T) {
	if _, err := Scan(filepath.Join(t.TempDir(), "nope")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("err = %v", err)
	}
}
