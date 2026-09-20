package vault

import (
	"bytes"
	"errors"
	"maps"
	"testing"
	"uuid"

	"github.com/elliot40404/creds/internal/crypto"
	"github.com/elliot40404/creds/internal/testutil"
	"github.com/elliot40404/creds/internal/vaultfiles"
)

func slotEntry(path string, slot byte) Entry {
	e := mergeEntry(path, "1")
	e.ID = idInSlot(int(slot))
	return e
}

func sealSet(t *testing.T, id *crypto.Identity, entries ...Entry) BucketSet {
	t.Helper()
	key, err := id.MACKey()
	if err != nil {
		t.Fatal(err)
	}
	slots := make([][]Entry, slotCount)
	for _, e := range entries {
		slots[SlotOf(e.ID)] = append(slots[SlotOf(e.ID)], e)
	}
	set := BucketSet{}
	for slot, list := range slots {
		if list == nil {
			list = []Entry{}
		}
		data, err := sealBucket(id.Recipient(), key, bucket{Format: bucketFormat, Slot: slot, Entries: list})
		if err != nil {
			t.Fatal(err)
		}
		set[vaultfiles.BucketName(slot)] = data
	}
	return set
}

func withBucket(set BucketSet, other BucketSet, slots ...int) BucketSet {
	out := maps.Clone(set)
	for _, s := range slots {
		out[vaultfiles.BucketName(s)] = other[vaultfiles.BucketName(s)]
	}
	return out
}

func openSet(t *testing.T, id *crypto.Identity, set BucketSet, slot int) []Entry {
	t.Helper()
	key, err := id.MACKey()
	if err != nil {
		t.Fatal(err)
	}
	b, err := openBucket(id, key, slot, set[vaultfiles.BucketName(slot)])
	if err != nil {
		t.Fatal(err)
	}
	return b.Entries
}

func mustMergeVault(t *testing.T, id *crypto.Identity, base, ours, theirs BucketSet, prefer map[uuid.UUID]Side) BucketSet {
	t.Helper()
	out, conflicts, err := MergeVault(id, base, ours, theirs, prefer)
	if err != nil || len(conflicts) != 0 {
		t.Fatalf("err %v conflicts %+v", err, conflicts)
	}
	return out
}

func TestMergeVaultSameBucket(t *testing.T) {
	id, _ := testutil.Keys(t)
	a, c := slotEntry("a", 3), slotEntry("c", 3)
	base := sealSet(t, id, a)
	ours := withBucket(base, sealSet(t, id, edited(a, "2")), 3)
	theirs := withBucket(base, sealSet(t, id, a, c), 3)
	out := mustMergeVault(t, id, base, ours, theirs, nil)
	if len(out) != 1 {
		t.Fatalf("changed buckets %d", len(out))
	}
	got := openSet(t, id, out, 3)
	if len(got) != 2 || got[0].Path != "a" || got[0].Fields[0].Value != "2" || got[1].ID != c.ID {
		t.Fatalf("got %+v", got)
	}
}

func TestMergeVaultTakesTheirsBucket(t *testing.T) {
	id, _ := testutil.Keys(t)
	base := sealSet(t, id)
	ours := withBucket(base, sealSet(t, id, slotEntry("a", 1)), 1)
	theirs := withBucket(base, sealSet(t, id, slotEntry("b", 2)), 2)
	out := mustMergeVault(t, id, base, ours, theirs, nil)
	if len(out) != 1 || !bytes.Equal(out[vaultfiles.BucketName(2)], theirs[vaultfiles.BucketName(2)]) {
		t.Fatalf("out %v", out)
	}
}

func TestMergeVaultConflict(t *testing.T) {
	id, _ := testutil.Keys(t)
	a := slotEntry("a", 5)
	base := sealSet(t, id, a)
	ours := sealSet(t, id, edited(a, "2"))
	theirs := sealSet(t, id, edited(a, "3"))
	out, conflicts, err := MergeVault(id, base, ours, theirs, nil)
	if err != nil || out != nil || len(conflicts) != 1 || conflicts[0].ID != a.ID {
		t.Fatalf("out %v conflicts %+v err %v", out, conflicts, err)
	}
	for side, want := range map[Side]string{Mine: "2", Theirs: "3"} {
		out := mustMergeVault(t, id, base, ours, theirs, map[uuid.UUID]Side{a.ID: side})
		if got := openSet(t, id, out, 5); len(got) != 1 || got[0].Fields[0].Value != want {
			t.Fatalf("side %d got %+v", side, got)
		}
	}
}

func TestMergeVaultCrossBucketPathClash(t *testing.T) {
	id, _ := testutil.Keys(t)
	x, y := slotEntry("same", 1), slotEntry("same", 2)
	base := sealSet(t, id)
	ours := withBucket(base, sealSet(t, id, x), 1)
	theirs := withBucket(base, sealSet(t, id, y), 2)
	_, conflicts, err := MergeVault(id, base, ours, theirs, nil)
	if err != nil || len(conflicts) != 1 || conflicts[0].Ours.ID != x.ID || conflicts[0].Theirs.ID != y.ID {
		t.Fatalf("conflicts %+v err %v", conflicts, err)
	}
	both := func(s Side) map[uuid.UUID]Side { return map[uuid.UUID]Side{x.ID: s, y.ID: s} }
	out := mustMergeVault(t, id, base, ours, theirs, both(Mine))
	if got := openSet(t, id, out, 2); len(got) != 0 {
		t.Fatalf("theirs entry kept %+v", got)
	}
	out = mustMergeVault(t, id, base, ours, theirs, both(Theirs))
	if got := openSet(t, id, out, 1); len(got) != 0 {
		t.Fatalf("ours entry kept %+v", got)
	}
	if got := openSet(t, id, out, 2); len(got) != 1 || got[0].ID != y.ID {
		t.Fatalf("theirs entry lost %+v", got)
	}
}

func TestMergeVaultRejectsForgedTheirs(t *testing.T) {
	id, _ := testutil.Keys(t)
	base := sealSet(t, id)
	forged, err := sealBucket(id.Recipient(), []byte("wrong key"), bucket{Format: bucketFormat, Slot: 4, Entries: []Entry{slotEntry("evil", 4)}})
	if err != nil {
		t.Fatal(err)
	}
	theirs := withBucket(base, BucketSet{vaultfiles.BucketName(4): forged}, 4)
	if _, _, err := MergeVault(id, base, base, theirs, nil); !errors.Is(err, ErrBadBucket) {
		t.Fatalf("want ErrBadBucket, got %v", err)
	}
}

func TestMergeVaultRejectsUnknownName(t *testing.T) {
	id, _ := testutil.Keys(t)
	base := sealSet(t, id)
	theirs := maps.Clone(base)
	theirs["zz.enc"] = []byte("x")
	if _, _, err := MergeVault(id, base, base, theirs, nil); !errors.Is(err, ErrBadBucket) {
		t.Fatalf("want ErrBadBucket, got %v", err)
	}
}
