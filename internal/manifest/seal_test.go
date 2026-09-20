package manifest

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/elliot40404/creds/internal/fsutil"
	"github.com/elliot40404/creds/internal/testutil"
	"github.com/elliot40404/creds/internal/vaultfiles"
)

func TestSealOpenRoundtrip(t *testing.T) {
	t.Parallel()
	id := testutil.Identity(t)
	m := good(t)
	data, err := Seal(id, m)
	if err != nil {
		t.Fatal(err)
	}
	back, err := Open(id, data)
	if err != nil {
		t.Fatal(err)
	}
	a, _ := m.ID()
	b, _ := back.ID()
	if a != b {
		t.Fatalf("%s != %s", a, b)
	}
}

func TestOpenRefusesAnotherIdentity(t *testing.T) {
	t.Parallel()
	data, err := Seal(testutil.Identity(t), good(t))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Open(testutil.Identity(t), data); err == nil {
		t.Fatal("opened with the wrong identity")
	}
}

func TestOpenRefusesOversize(t *testing.T) {
	t.Parallel()
	if _, err := Open(testutil.Identity(t), make([]byte, MaxSize+1)); !errors.Is(err, fsutil.ErrTooLarge) {
		t.Fatalf("err = %v", err)
	}
}

func TestManifestKeyDiffersFromBucketKey(t *testing.T) {
	t.Parallel()
	id := testutil.Identity(t)
	bucket, err := id.MACKey()
	if err != nil {
		t.Fatal(err)
	}
	man, err := id.ManifestMACKey()
	if err != nil {
		t.Fatal(err)
	}
	if string(bucket) == string(man) {
		t.Fatal("manifest mac key equals the bucket mac key")
	}
}

func fakeVault(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := fsutil.EnsureDir(filepath.Join(dir, vaultfiles.EntriesDir)); err != nil {
		t.Fatal(err)
	}
	for _, p := range vaultfiles.Covered() {
		path := filepath.Join(dir, filepath.FromSlash(p))
		if err := os.WriteFile(path, []byte("body of "+p), fsutil.FilePerm); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestWriteThenVerify(t *testing.T) {
	t.Parallel()
	dir, id := fakeVault(t), testutil.Identity(t)
	m, err := Write(dir, id, nil)
	if err != nil {
		t.Fatal(err)
	}
	if m.Generation != 1 {
		t.Fatalf("gen %d", m.Generation)
	}
	got, err := Verify(dir, id)
	if err != nil {
		t.Fatal(err)
	}
	a, _ := m.ID()
	b, _ := got.ID()
	if a != b {
		t.Fatalf("%s != %s", a, b)
	}
}

func TestVerifyCatchesATamperedFile(t *testing.T) {
	t.Parallel()
	dir, id := fakeVault(t), testutil.Identity(t)
	if _, err := Write(dir, id, nil); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, vaultfiles.EntriesDir, vaultfiles.BucketName(3))
	if err := os.WriteFile(path, []byte("tampered"), fsutil.FilePerm); err != nil {
		t.Fatal(err)
	}
	if _, err := Verify(dir, id); !errors.Is(err, ErrBadManifest) {
		t.Fatalf("err = %v", err)
	}
}

func TestVerifyCatchesAReplayedFile(t *testing.T) {
	t.Parallel()
	dir, id := fakeVault(t), testutil.Identity(t)
	path := filepath.Join(dir, vaultfiles.EntriesDir, vaultfiles.BucketName(3))
	old := []byte("body of " + vaultfiles.EntriesDir + "/" + vaultfiles.BucketName(3))
	if _, err := Write(dir, id, nil); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("newer"), fsutil.FilePerm); err != nil {
		t.Fatal(err)
	}
	if _, err := Verify(dir, id); err == nil {
		t.Fatal("stale manifest accepted")
	}
	first, err := Load(dir, id)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Write(dir, id, []Manifest{first})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, old, fsutil.FilePerm); err != nil {
		t.Fatal(err)
	}
	if _, err := Verify(dir, id); !errors.Is(err, ErrBadManifest) {
		t.Fatalf("replayed bucket accepted at generation %d: %v", second.Generation, err)
	}
}
