package vault

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"
	"uuid"

	"github.com/elliot40404/creds/internal/crypto"
	"github.com/elliot40404/creds/internal/format"
	"github.com/elliot40404/creds/internal/fsutil"
	"github.com/elliot40404/creds/internal/testutil"
	"github.com/elliot40404/creds/internal/vaultfiles"
)

func initVault(t *testing.T) (*Vault, *crypto.Identity, string) {
	t.Helper()
	id, _ := testutil.Keys(t)
	dir := filepath.Join(t.TempDir(), "vault")
	v, err := Init(dir, id)
	if err != nil {
		t.Fatal(err)
	}
	return v, id, dir
}

func mustPut(t *testing.T, v *Vault, e Entry) Entry {
	t.Helper()
	got, err := v.Put(e)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func TestInitWritesAllBuckets(t *testing.T) {
	_, id, dir := initVault(t)
	for slot := range slotCount {
		if _, err := os.Stat(filepath.Join(dir, vaultfiles.EntriesDir, vaultfiles.BucketName(slot))); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := Init(dir, id); !errors.Is(err, os.ErrExist) {
		t.Fatalf("got %v", err)
	}
	v, err := Load(dir, id)
	if err != nil {
		t.Fatal(err)
	}
	if len(v.List()) != 0 {
		t.Fatal("expected empty vault")
	}
}

func TestCRUD(t *testing.T) {
	v, id, dir := initVault(t)
	a := mustPut(t, v, entryInSlot(1, "work/a"))
	b := sampleEntry()
	b.ID = uuid.Nil()
	b.Path = "work/b"
	b = mustPut(t, v, b)
	if b.ID == a.ID || b.Created.IsZero() {
		t.Fatalf("bad new entry %+v", b)
	}
	a.Notes = "changed"
	mustPut(t, v, a)
	if err := v.Save(); err != nil {
		t.Fatal(err)
	}
	r, err := Load(dir, id)
	if err != nil {
		t.Fatal(err)
	}
	got, err := r.Get("work/a")
	if err != nil || got.Notes != "changed" || got.ID != a.ID {
		t.Fatalf("got %+v %v", got, err)
	}
	if err := r.Delete("work/b"); err != nil {
		t.Fatal(err)
	}
	if err := r.Save(); err != nil {
		t.Fatal(err)
	}
	r, err = Load(dir, id)
	if err != nil {
		t.Fatal(err)
	}
	paths := []string{}
	for _, e := range r.List() {
		paths = append(paths, e.Path)
	}
	if !slices.Equal(paths, []string{"work/a"}) {
		t.Fatalf("got %v", paths)
	}
	if _, err := r.Get("work/b"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("got %v", err)
	}
	if err := r.Delete("work/b"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("got %v", err)
	}
}

func TestGetReturnsCopy(t *testing.T) {
	v, _, _ := initVault(t)
	mustPut(t, v, entryInSlot(0, "a"))
	e, _ := v.Get("a")
	e.Tags[0] = "mutated"
	again, _ := v.Get("a")
	if again.Tags[0] != "prod" {
		t.Fatal("vault state mutated through Get")
	}
}

func TestPutRejects(t *testing.T) {
	v, _, _ := initVault(t)
	a := mustPut(t, v, entryInSlot(0, "a"))
	if _, err := v.Put(entryInSlot(0, "a")); !errors.Is(err, ErrDuplicatePath) {
		t.Fatalf("got %v", err)
	}
	a.Path = "b"
	if _, err := v.Put(a); !errors.Is(err, ErrDuplicateID) {
		t.Fatalf("got %v", err)
	}
	if _, err := v.Put(entryInSlot(0, "")); !errors.Is(err, ErrInvalidEntry) {
		t.Fatalf("got %v", err)
	}
}

func recordWrites(t *testing.T, fail string) *[]string {
	t.Helper()
	var names []string
	t.Cleanup(func() { writeFile = fsutil.WriteFileAtomic })
	writeFile = func(path string, data []byte) error {
		name := filepath.Base(path)
		names = append(names, name)
		if name == fail {
			return errors.New("disk full")
		}
		return fsutil.WriteFileAtomic(path, data)
	}
	return &names
}

func TestSaveOnlyDirty(t *testing.T) {
	v, _, _ := initVault(t)
	names := recordWrites(t, "")
	mustPut(t, v, entryInSlot(2, "a"))
	mustPut(t, v, entryInSlot(10, "b"))
	if err := v.Save(); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(*names, []string{"02.enc", "0a.enc"}) {
		t.Fatalf("got %v", *names)
	}
	if err := v.Save(); err != nil {
		t.Fatal(err)
	}
	if len(*names) != 2 {
		t.Fatalf("clean save wrote %v", *names)
	}
}

func TestSaveFailureKeepsOld(t *testing.T) {
	v, id, dir := initVault(t)
	mustPut(t, v, entryInSlot(3, "old"))
	if err := v.Save(); err != nil {
		t.Fatal(err)
	}
	recordWrites(t, "03.enc")
	mustPut(t, v, entryInSlot(3, "new"))
	if err := v.Save(); err == nil {
		t.Fatal("expected write error")
	}
	if !v.dirty[3] {
		t.Fatal("failed slot marked clean")
	}
	r, err := Load(dir, id)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.List()) != 1 {
		t.Fatalf("got %v", r.List())
	}
	writeFile = fsutil.WriteFileAtomic
	if err := v.Save(); err != nil {
		t.Fatal(err)
	}
}

func TestLoadRejectsSwappedBucket(t *testing.T) {
	v, id, dir := initVault(t)
	mustPut(t, v, entryInSlot(1, "a"))
	if err := v.Save(); err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(filepath.Join(dir, vaultfiles.EntriesDir))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = root.Close() }()
	data, err := root.ReadFile("01.enc")
	if err != nil {
		t.Fatal(err)
	}
	if err := root.WriteFile("02.enc", data, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(dir, id); !errors.Is(err, ErrBadBucket) {
		t.Fatalf("got %v", err)
	}
}

func TestLoadRejectsForgedBucket(t *testing.T) {
	_, id, dir := initVault(t)
	_, forgerKey := testutil.Keys(t)
	e := entryInSlot(5, "evil")
	data, err := sealBucket(id.Recipient(), forgerKey, bucket{Format: bucketFormat, Slot: 5, Entries: []Entry{e}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, vaultfiles.EntriesDir, "05.enc"), data, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(dir, id); !errors.Is(err, ErrBadBucket) {
		t.Fatalf("got %v", err)
	}
}

func TestLoadRejectsRecipientSwap(t *testing.T) {
	_, id, dir := initVault(t)
	other, _ := testutil.Keys(t)
	path := filepath.Join(dir, vaultfiles.MetaFile)
	m, err := format.LoadMeta(path)
	if err != nil {
		t.Fatal(err)
	}
	m.Recipient = other.Recipient().String()
	if err := format.SaveMeta(path, m); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(dir, id); !errors.Is(err, ErrRecipientMismatch) {
		t.Fatalf("got %v", err)
	}
}

func TestLoadMissingBucket(t *testing.T) {
	_, id, dir := initVault(t)
	if err := os.Remove(filepath.Join(dir, vaultfiles.EntriesDir, "07.enc")); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(dir, id); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("got %v", err)
	}
}

func TestLoadRejectsSymlinkedBucket(t *testing.T) {
	_, id, dir := initVault(t)
	other := filepath.Join(dir, vaultfiles.EntriesDir, "08.enc")
	path := filepath.Join(dir, vaultfiles.EntriesDir, "07.enc")
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(other, path); err != nil {
		t.Skip("symlinks unavailable:", err)
	}
	if _, err := Load(dir, id); err == nil {
		t.Fatal("symlinked bucket accepted")
	}
}

func TestPutStoresUTC(t *testing.T) {
	v, id, dir := initVault(t)
	zone := time.FixedZone("IST", 5*3600+1800)
	v.now = func() time.Time { return time.Date(2026, 9, 17, 10, 0, 0, 0, zone) }
	e := entryInSlot(0, "a")
	e.Created = time.Date(2026, 1, 1, 8, 0, 0, 0, zone)
	e.ID = uuid.Nil()
	got := mustPut(t, v, e)
	e = entryInSlot(1, "b")
	e.Created = time.Date(2026, 1, 1, 8, 0, 0, 0, zone)
	kept := mustPut(t, v, e)
	if err := v.Save(); err != nil {
		t.Fatal(err)
	}
	re, err := Load(dir, id)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []Entry{got, kept} {
		back, err := re.Get(want.Path)
		if err != nil {
			t.Fatal(err)
		}
		for _, ts := range []time.Time{want.Created, want.Updated, back.Created, back.Updated} {
			if ts.Location() != time.UTC {
				t.Fatalf("%s: time %v not UTC", want.Path, ts)
			}
		}
	}
	if !kept.Created.Equal(time.Date(2026, 1, 1, 2, 30, 0, 0, time.UTC)) {
		t.Fatalf("created %v", kept.Created)
	}
}

func TestPutRejectsBadPath(t *testing.T) {
	v, _, _ := initVault(t)
	if _, err := v.Put(entryInSlot(0, "../etc/passwd")); !errors.Is(err, ErrBadPath) {
		t.Fatalf("got %v", err)
	}
	if len(v.List()) != 0 {
		t.Fatal("bad path stored")
	}
}

func TestCloseWipesTheMACKey(t *testing.T) {
	v, _, _ := initVault(t)
	key := v.key
	v.Close()
	if slices.ContainsFunc(key, func(b byte) bool { return b != 0 }) {
		t.Fatal("mac key still in memory")
	}
}
