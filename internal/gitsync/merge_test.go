package gitsync

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"uuid"

	"github.com/elliot40404/creds/internal/crypto"
	"github.com/elliot40404/creds/internal/manifest"
	"github.com/elliot40404/creds/internal/testutil"
	"github.com/elliot40404/creds/internal/vault"
	"github.com/elliot40404/creds/internal/vaultfiles"
)

type vaultPair struct {
	bare string
	a    *Repo
	b    *Repo
	id   *crypto.Identity
}

func newVaultPair(t *testing.T) vaultPair {
	t.Helper()
	id := testutil.Identity(t)
	bare, a, b := pushedPair(t, func(a *Repo) {
		if _, err := vault.Init(a.Dir, id); err != nil {
			t.Fatal(err)
		}
		writeFile(t, a, "identity.pw.age", ageBlob)
		writeFile(t, a, "identity.recovery.age", ageBlob)
		sealManifest(t, a, id)
	}, SyncOptions{Identity: id})
	return vaultPair{bare: bare, a: a, b: b, id: id}
}

func (p vaultPair) opts() SyncOptions {
	return SyncOptions{Identity: p.id}
}

func (p vaultPair) sync(t *testing.T, r *Repo, want SyncResult) {
	t.Helper()
	got, err := r.Sync(context.Background(), p.opts())
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("sync %v, want %v", got, want)
	}
}

func (p vaultPair) edit(t *testing.T, r *Repo, fn func(v *vault.Vault) error) {
	t.Helper()
	v, err := vault.Load(r.Dir, p.id)
	if err != nil {
		t.Fatal(err)
	}
	if err := fn(v); err != nil {
		t.Fatal(err)
	}
	if err := v.Save(); err != nil {
		t.Fatal(err)
	}
	sealManifest(t, r, p.id)
	if !mustCommit(t, r, "edit") {
		t.Fatal("nothing committed")
	}
}

func sealManifest(t *testing.T, r *Repo, id *crypto.Identity) {
	t.Helper()
	var parents []manifest.Manifest
	prev, err := manifest.Load(r.Dir, id)
	switch {
	case err == nil:
		parents = []manifest.Manifest{prev}
	case !errors.Is(err, fs.ErrNotExist):
		t.Fatal(err)
	}
	if _, err := manifest.Write(r.Dir, id, parents); err != nil {
		t.Fatal(err)
	}
}

func commitPasswordFile(t *testing.T, p vaultPair, r *Repo) {
	t.Helper()
	name := vaultfiles.PasswordFile
	writeFile(t, r, name, ageBlob+name)
	sealManifest(t, r, p.id)
	if !mustCommit(t, r, name) {
		t.Fatal("nothing committed")
	}
}

func (p vaultPair) put(t *testing.T, r *Repo, e vault.Entry) {
	t.Helper()
	p.edit(t, r, func(v *vault.Vault) error {
		_, err := v.Put(e)
		return err
	})
}

func (p vaultPair) password(t *testing.T, r *Repo, path string) string {
	t.Helper()
	v, err := vault.Load(r.Dir, p.id)
	if err != nil {
		t.Fatal(err)
	}
	e, err := v.Get(path)
	if err != nil {
		t.Fatal(err)
	}
	return e.Fields[0].Value
}

func (p vaultPair) conflicts(t *testing.T, r *Repo, opts SyncOptions) []Conflict {
	t.Helper()
	head := gitOut(t, r, "rev-parse", "HEAD")
	remote := bareHead(t, p.bare)
	_, err := r.Sync(context.Background(), opts)
	cerr, ok := errors.AsType[*ConflictError](err)
	if !ok || !errors.Is(err, ErrConflict) {
		t.Fatalf("want conflict, got %v", err)
	}
	if gitOut(t, r, "rev-parse", "HEAD") != head || bareHead(t, p.bare) != remote {
		t.Fatal("refs changed on conflict")
	}
	if dirty(t, r) {
		t.Fatal("working tree changed on conflict")
	}
	return cerr.Conflicts
}

func secretEntry(path, pw string) vault.Entry {
	return vault.Entry{
		ID:     uuid.NewV7(),
		Path:   path,
		Type:   vault.TypeLogin,
		Fields: []vault.Field{{Name: "password", Value: pw, Secret: true}},
	}
}

func idNearSlot(slot int, same bool) uuid.UUID {
	for {
		id := uuid.NewV7()
		if (vault.SlotOf(id) == slot) == same {
			return id
		}
	}
}

func setPassword(pw string) func(v *vault.Vault) error {
	return func(v *vault.Vault) error {
		e, err := v.Get("e")
		if err != nil {
			return err
		}
		e.Fields[0].Value = pw
		_, err = v.Put(e)
		return err
	}
}

func TestMergeDifferentEntriesSameBucket(t *testing.T) {
	t.Parallel()
	p := newVaultPair(t)
	x, y := secretEntry("x", "1"), secretEntry("y", "2")
	y.ID = idNearSlot(vault.SlotOf(x.ID), true)
	p.put(t, p.a, x)
	p.sync(t, p.a, Pushed)
	p.put(t, p.b, y)
	p.sync(t, p.b, Merged)
	if n := gitOut(t, p.b, "rev-list", "--count", "--merges", "HEAD"); n != "1" {
		t.Fatalf("merge commits %s", n)
	}
	p.sync(t, p.a, FastForwarded)
	for _, r := range []*Repo{p.a, p.b} {
		if p.password(t, r, "x") != "1" || p.password(t, r, "y") != "2" {
			t.Fatal("entry lost")
		}
		if st := mustStatus(t, r); dirty(t, r) || st.Ahead != 0 || st.Behind != 0 {
			t.Fatalf("status %+v", st)
		}
	}
}

func TestMergeEntryConflictResolve(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		side vault.Side
		want string
	}{{vault.Mine, "b"}, {vault.Theirs, "a"}} {
		p := newVaultPair(t)
		e := secretEntry("e", "0")
		p.put(t, p.a, e)
		p.sync(t, p.a, Pushed)
		p.sync(t, p.b, FastForwarded)
		p.edit(t, p.a, setPassword("a"))
		p.sync(t, p.a, Pushed)
		p.edit(t, p.b, setPassword("b"))
		got := p.conflicts(t, p.b, p.opts())
		if len(got) != 1 || got[0].Entry == nil || got[0].Entry.ID != e.ID {
			t.Fatalf("conflicts %+v", got)
		}
		opts := p.opts()
		opts.PreferEntry = map[uuid.UUID]vault.Side{e.ID: tc.side}
		if res, err := p.b.Sync(context.Background(), opts); err != nil || res != Merged {
			t.Fatalf("resolve %v %v", res, err)
		}
		p.sync(t, p.a, FastForwarded)
		if p.password(t, p.a, "e") != tc.want || p.password(t, p.b, "e") != tc.want {
			t.Fatalf("side %d: want %s", tc.side, tc.want)
		}
	}
}

func TestMergeDeleteVsEditConflict(t *testing.T) {
	t.Parallel()
	p := newVaultPair(t)
	p.put(t, p.a, secretEntry("e", "0"))
	p.sync(t, p.a, Pushed)
	p.sync(t, p.b, FastForwarded)
	p.edit(t, p.a, func(v *vault.Vault) error { return v.Delete("e") })
	p.sync(t, p.a, Pushed)
	p.edit(t, p.b, setPassword("b"))
	got := p.conflicts(t, p.b, p.opts())
	if len(got) != 1 || got[0].Entry.Ours == nil || got[0].Entry.Theirs != nil {
		t.Fatalf("conflicts %+v", got)
	}
}

func TestMergeIdentityFileConflict(t *testing.T) {
	t.Parallel()
	p := newVaultPair(t)
	commitPasswordFile(t, p, p.a)
	p.sync(t, p.a, Pushed)
	writeFile(t, p.b, "identity.pw.age", ageBlob+"other")
	sealManifest(t, p.b, p.id)
	mustCommit(t, p.b, "passwd")
	got := p.conflicts(t, p.b, p.opts())
	if len(got) != 1 || got[0].File != "identity.pw.age" || got[0].Entry != nil {
		t.Fatalf("conflicts %+v", got)
	}
	opts := p.opts()
	opts.PreferFile = map[string]vault.Side{"identity.pw.age": vault.Theirs}
	if res, err := p.b.Sync(context.Background(), opts); err != nil || res != Merged {
		t.Fatalf("resolve %v %v", res, err)
	}
	data, err := os.ReadFile(filepath.Join(p.b.Dir, "identity.pw.age"))
	if err != nil || string(data) != ageBlob+"identity.pw.age" {
		t.Fatalf("identity %q %v", data, err)
	}
}

func TestMergeRejectsForgedBucket(t *testing.T) {
	t.Parallel()
	p := newVaultPair(t)
	forged, err := crypto.Seal(p.id.Recipient(), []byte(`{"mac":"00","bucket":{"format":1,"slot":5,"entries":[]}}`))
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, p.a, "entries/05.enc", string(forged))
	sealManifest(t, p.a, p.id)
	mustCommit(t, p.a, "forged")
	p.sync(t, p.a, Pushed)
	e := secretEntry("mine", "1")
	e.ID = idNearSlot(5, false)
	p.put(t, p.b, e)
	head := gitOut(t, p.b, "rev-parse", "HEAD")
	if _, err := p.b.Sync(context.Background(), p.opts()); !errors.Is(err, vault.ErrBadBucket) {
		t.Fatalf("want ErrBadBucket, got %v", err)
	}
	if gitOut(t, p.b, "rev-parse", "HEAD") != head {
		t.Fatal("head moved")
	}
}

func TestMergeRejectsDisallowedRemotePath(t *testing.T) {
	t.Parallel()
	p := newVaultPair(t)
	writeFile(t, p.a, "notes.txt", "x")
	gitOut(t, p.a, "add", "-f", "notes.txt")
	gitOut(t, p.a, "commit", "-q", "-m", "bad")
	gitOut(t, p.a, "push", "-q", Remote, Branch)
	p.put(t, p.b, secretEntry("mine", "1"))
	if _, err := p.b.Sync(context.Background(), p.opts()); !errors.Is(err, ErrNotAllowed) {
		t.Fatalf("want ErrNotAllowed, got %v", err)
	}
}
