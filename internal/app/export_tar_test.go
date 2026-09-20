package app

import (
	"archive/tar"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/elliot40404/creds/internal/config"
	"github.com/elliot40404/creds/internal/fsutil"
	"github.com/elliot40404/creds/internal/gitsync"
	"github.com/elliot40404/creds/internal/vault"
	"github.com/elliot40404/creds/internal/vaultfiles"
)

func allEntries(t *testing.T, s *Service) []vault.Entry {
	t.Helper()
	v, err := s.open()
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	return v.List()
}

func untar(t *testing.T, path, dir string) []string {
	t.Helper()
	f, err := os.Open(filepath.Clean(path))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = root.Close() }()
	var names []string
	r := tar.NewReader(f)
	for {
		hdr, err := r.Next()
		if errors.Is(err, io.EOF) {
			return names
		}
		if err != nil {
			t.Fatalf("tar: %v", err)
		}
		names = append(names, hdr.Name)
		data, err := io.ReadAll(r)
		if err != nil {
			t.Fatal(err)
		}
		if err := root.MkdirAll(filepath.Dir(hdr.Name), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := root.WriteFile(hdr.Name, data, fsutil.FilePerm); err != nil {
			t.Fatal(err)
		}
	}
}

func TestExportEncrypted(t *testing.T) {
	t.Parallel()
	s, fp := exportVault(t)
	vdir := s.Paths.Vault()
	for _, junk := range []string{"entries/notes.txt", "secret.txt"} {
		if err := os.WriteFile(filepath.Join(vdir, junk), []byte("plain"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	for _, junk := range []string{s.Paths.State(), s.Paths.Config()} {
		if err := os.WriteFile(junk, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	out := filepath.Join(t.TempDir(), "backup.tar")
	n, err := s.ExportEncrypted(out, false)
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	if len(fp.prompts) != 0 {
		t.Fatalf("prompted %v", fp.prompts)
	}
	home := t.TempDir()
	restored := config.Paths{Home: home}
	if err := os.MkdirAll(restored.Vault(), 0o700); err != nil {
		t.Fatal(err)
	}
	names := untar(t, out, restored.Vault())
	if len(names) != n || !slices.Contains(names, "vault.json") || !slices.Contains(names, "identity.recovery.age") {
		t.Fatalf("names %v", names)
	}
	for _, name := range names {
		if !vaultfiles.Allowed(name) {
			t.Fatalf("unexpected %s", name)
		}
	}
	dst := &Service{Paths: restored, Config: s.Config, Prompter: fp, Now: s.Now, LogN: s.LogN}
	fp.passwords = []string{mainWord}
	if !reflect.DeepEqual(allEntries(t, dst), allEntries(t, s)) {
		t.Fatal("restored vault differs")
	}
	e, err := dst.Get("shop/db")
	if err != nil || e.Fields[0].Value != entrySecret {
		t.Fatalf("restored %+v %v", e, err)
	}
	if _, err := s.ExportEncrypted(out, false); !errors.Is(err, ErrFileExists) {
		t.Fatalf("existing err %v", err)
	}
}

func TestExportEncryptedNoVault(t *testing.T) {
	t.Parallel()
	s, _, _ := newService(t)
	if _, err := s.ExportEncrypted(filepath.Join(t.TempDir(), "b.tar"), false); !errors.Is(err, ErrNoVault) {
		t.Fatalf("err %v", err)
	}
}

func TestBackupNamesMatchGitTree(t *testing.T) {
	t.Parallel()
	s, _, _, _ := initVault(t)
	for _, name := range []string{"notes.txt", "entries/zz.enc", "entries/0F.enc"} {
		if err := os.WriteFile(filepath.Join(s.Paths.Vault(), filepath.FromSlash(name)), []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	out, err := gitsync.NewGit(s.Paths.Vault()).Run(context.Background(), "ls-files")
	if err != nil {
		t.Fatal(err)
	}
	tracked := strings.Fields(string(out))
	root, err := os.OpenRoot(s.Paths.Vault())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = root.Close() }()
	names, err := backupNames(root)
	if err != nil {
		t.Fatal(err)
	}
	slices.Sort(tracked)
	slices.Sort(names)
	if !slices.Equal(names, tracked) {
		t.Fatalf("tar %v git %v", names, tracked)
	}
}
