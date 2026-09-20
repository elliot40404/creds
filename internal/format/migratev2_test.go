package format

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/elliot40404/creds/internal/vaultfiles"
)

func copyV2(t *testing.T) string {
	t.Helper()
	return copyFixture(t, "v2", map[string]string{"vault.json": vaultfiles.MetaFile, "gitignore.txt": vaultfiles.IgnoreFile})
}

func TestMigrateV2KeepsMetaAndOpensTheIgnoreFile(t *testing.T) {
	dir := copyV2(t)
	from, err := Migrate(dir)
	if err != nil {
		t.Fatal(err)
	}
	if from != 2 {
		t.Fatalf("from %d", from)
	}
	m, err := LoadMeta(filepath.Join(dir, vaultfiles.MetaFile))
	if err != nil {
		t.Fatal(err)
	}
	want := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	if m.FormatVersion != 3 || m.Recipient != "age1pq1example" || !m.Created.Equal(want) {
		t.Fatalf("got %+v", m)
	}
	if got := readString(t, filepath.Join(dir, vaultfiles.IgnoreFile)); got != vaultfiles.IgnoreRules {
		t.Fatalf("ignore file %q", got)
	}
}

func TestMigrateV2IsIdempotent(t *testing.T) {
	dir := copyV2(t)
	if _, err := Migrate(dir); err != nil {
		t.Fatal(err)
	}
	meta := readString(t, filepath.Join(dir, vaultfiles.MetaFile))
	from, err := Migrate(dir)
	if err != nil || from != 3 {
		t.Fatalf("from %d err %v", from, err)
	}
	if got := readString(t, filepath.Join(dir, vaultfiles.MetaFile)); got != meta {
		t.Fatalf("meta rewritten %q", got)
	}
}

func TestFailedV2UpgradeLeavesV2Usable(t *testing.T) {
	dir := copyV2(t)
	before := readString(t, filepath.Join(dir, vaultfiles.MetaFile))
	ignore := readString(t, filepath.Join(dir, vaultfiles.IgnoreFile))
	boom := errors.New("boom")
	withUpgrades(t, []upgrade{upgradeV0, upgradeV1, func(t *txn) error {
		if err := t.write(vaultfiles.IgnoreFile, []byte(vaultfiles.IgnoreRules)); err != nil {
			return err
		}
		return boom
	}})
	if _, err := Migrate(dir); !errors.Is(err, boom) {
		t.Fatalf("err = %v", err)
	}
	if got := readString(t, filepath.Join(dir, vaultfiles.MetaFile)); got != before {
		t.Fatalf("meta changed %q", got)
	}
	if got := readString(t, filepath.Join(dir, vaultfiles.IgnoreFile)); got != ignore {
		t.Fatalf("ignore file changed %q", got)
	}
	if _, err := readMeta(filepath.Join(dir, vaultfiles.MetaFile)); err != nil {
		t.Fatal(err)
	}
}
