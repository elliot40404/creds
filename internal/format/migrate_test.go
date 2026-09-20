package format

import (
	"errors"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"testing"
	"time"

	"github.com/elliot40404/creds/internal/vaultfiles"
)

func copyFixture(t *testing.T, version string, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for src, dst := range files {
		data, err := fs.ReadFile(os.DirFS("testdata"), path.Join(version, src))
		if err != nil {
			t.Fatal(err)
		}
		writeFile(t, filepath.Join(dir, dst), string(data))
	}
	return dir
}

func copyV0(t *testing.T) string {
	t.Helper()
	return copyFixture(t, "v0", map[string]string{vaultfiles.MetaFile: vaultfiles.MetaFile})
}

func withUpgrades(t *testing.T, m []upgrade) {
	t.Helper()
	old := upgrades
	upgrades = m
	t.Cleanup(func() { upgrades = old })
}

func readString(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestMigrateV0(t *testing.T) {
	dir := copyV0(t)
	from, err := Migrate(dir)
	if err != nil {
		t.Fatal(err)
	}
	if from != 0 {
		t.Fatalf("from %d", from)
	}
	m, err := LoadMeta(filepath.Join(dir, vaultfiles.MetaFile))
	if err != nil {
		t.Fatal(err)
	}
	want := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	if m.FormatVersion != CurrentVersion || m.Recipient != "age1pq1example" || !m.Created.Equal(want) {
		t.Fatalf("got %+v", m)
	}
	if _, err := os.Stat(filepath.Join(dir, backupDir)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("backup left behind: %v", err)
	}
	from, err = Migrate(dir)
	if err != nil || from != CurrentVersion {
		t.Fatalf("second run: %d %v", from, err)
	}
}

func TestMigrateNewerRefused(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, vaultfiles.MetaFile)
	writeFile(t, path, `{"format_version":9}`)
	if _, err := Migrate(dir); !errors.Is(err, ErrUnknownVersion) {
		t.Fatalf("got %v", err)
	}
	if readString(t, path) != `{"format_version":9}` {
		t.Fatal("file changed")
	}
}

func TestMigrateFailureRollsBack(t *testing.T) {
	dir := copyV0(t)
	metaPath := filepath.Join(dir, vaultfiles.MetaFile)
	before := readString(t, metaPath)
	boom := errors.New("boom")
	withUpgrades(t, []upgrade{func(tx *txn) error {
		if err := tx.write(vaultfiles.MetaFile, []byte(`{"format_version":1}`)); err != nil {
			return err
		}
		if err := tx.write(filepath.Join("entries", "00.enc"), []byte("new")); err != nil {
			return err
		}
		return boom
	}})
	if _, err := Migrate(dir); !errors.Is(err, boom) || err.Error() != "upgrade from version 0: boom" {
		t.Fatalf("got %v", err)
	}
	if readString(t, metaPath) != before {
		t.Fatal("meta not restored")
	}
	for _, p := range []string{filepath.Join(dir, "entries", "00.enc"), filepath.Join(dir, backupDir)} {
		if _, err := os.Stat(p); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("%s left behind: %v", p, err)
		}
	}
}

func TestMigrateRestoresAfterCrash(t *testing.T) {
	dir := copyV0(t)
	metaPath := filepath.Join(dir, vaultfiles.MetaFile)
	before := readString(t, metaPath)
	if err := os.MkdirAll(filepath.Join(dir, backupDir), 0o700); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(dir, backupDir, vaultfiles.MetaFile), before)
	writeFile(t, metaPath, `{"format_version":1,"half":true}`)
	if _, err := Migrate(dir); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadMeta(metaPath); err != nil {
		t.Fatal(err)
	}
}

func TestTxRejectsBadPaths(t *testing.T) {
	tx := newTx(t.TempDir())
	for _, name := range []string{"../x", "", backupDir, filepath.Join(backupDir, vaultfiles.MetaFile)} {
		if err := tx.write(name, nil); err == nil {
			t.Fatalf("%q: want error", name)
		}
	}
}

func symlinkOrSkip(t *testing.T, target, link string) {
	t.Helper()
	if err := os.Symlink(target, link); err != nil {
		t.Skip("symlinks unavailable:", err)
	}
}

func assertBadBackup(t *testing.T, dir string) {
	t.Helper()
	if _, err := Migrate(dir); !errors.Is(err, errBadBackup) {
		t.Fatalf("want errBadBackup, got %v", err)
	}
}

func TestRestoreRefusesSymlinkEscape(t *testing.T) {
	dir := copyV0(t)
	outside := t.TempDir()
	symlinkOrSkip(t, outside, filepath.Join(dir, "x"))
	writeFile(t, filepath.Join(dir, backupDir, "x", "pwned.txt"), "evil")
	assertBadBackup(t, dir)
	if _, err := os.Lstat(filepath.Join(outside, "pwned.txt")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("file written outside: %v", err)
	}
}

func TestRestoreRefusesUnexpectedNames(t *testing.T) {
	for _, name := range []string{"evil.sh", filepath.Join("entries", "zz.enc"), filepath.Join("sub", vaultfiles.MetaFile)} {
		t.Run(name, func(t *testing.T) {
			dir := copyV0(t)
			writeFile(t, filepath.Join(dir, backupDir, name), "evil")
			assertBadBackup(t, dir)
			if _, err := os.Lstat(filepath.Join(dir, name)); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("%s restored: %v", name, err)
			}
		})
	}
}

func TestRestoreRefusesBackupFile(t *testing.T) {
	dir := copyV0(t)
	writeFile(t, filepath.Join(dir, backupDir), "evil")
	assertBadBackup(t, dir)
}

func TestRestoreRefusesSymlinkedFile(t *testing.T) {
	dir := copyV0(t)
	secret := filepath.Join(t.TempDir(), "secret")
	writeFile(t, secret, "secret")
	if err := os.MkdirAll(filepath.Join(dir, backupDir), 0o700); err != nil {
		t.Fatal(err)
	}
	symlinkOrSkip(t, secret, filepath.Join(dir, backupDir, vaultfiles.PasswordFile))
	assertBadBackup(t, dir)
	if _, err := os.Lstat(filepath.Join(dir, vaultfiles.PasswordFile)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("symlink target restored: %v", err)
	}
}

func TestRestoreSymlinkedBackupDir(t *testing.T) {
	dir := copyV0(t)
	outside := t.TempDir()
	writeFile(t, filepath.Join(outside, vaultfiles.PasswordFile), "evil")
	symlinkOrSkip(t, outside, filepath.Join(dir, backupDir))
	assertBadBackup(t, dir)
}

func TestMigrateV1KeepsEverythingButTheVersion(t *testing.T) {
	dir := t.TempDir()
	created := "2026-01-02T03:04:05Z"
	writeFile(t, filepath.Join(dir, vaultfiles.MetaFile),
		`{"format_version":1,"recipient":"age1pq1example","created":"`+created+`"}`)
	from, err := Migrate(dir)
	if err != nil {
		t.Fatal(err)
	}
	if from != 1 {
		t.Fatalf("from %d", from)
	}
	m, err := LoadMeta(filepath.Join(dir, vaultfiles.MetaFile))
	if err != nil {
		t.Fatal(err)
	}
	want := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	if m.FormatVersion != CurrentVersion || m.Recipient != "age1pq1example" || !m.Created.Equal(want) {
		t.Fatalf("got %+v", m)
	}
}

func TestOldReaderRefusesV3(t *testing.T) {
	path := filepath.Join(t.TempDir(), vaultfiles.MetaFile)
	writeFile(t, path, `{"format_version":3,"recipient":"age1pq1example","created":"2026-01-02T03:04:05Z"}`)
	for _, old := range []int{1, 2} {
		if err := checkVersion(3, old); !errors.Is(err, ErrUnknownVersion) {
			t.Fatalf("a v%d reader got %v, want %v", old, err, ErrUnknownVersion)
		}
	}
	if _, err := LoadMeta(path); err != nil {
		t.Fatalf("this reader should load v3: %v", err)
	}
}

func TestEveryVersionHasAnUpgrade(t *testing.T) {
	if len(upgrades) != CurrentVersion {
		t.Fatalf("%d upgrades for version %d", len(upgrades), CurrentVersion)
	}
}
