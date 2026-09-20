package vault

import (
	"encoding/json/v2"
	"errors"
	"testing"
	"uuid"

	"github.com/elliot40404/creds/internal/crypto"
	"github.com/elliot40404/creds/internal/testutil"
	"github.com/elliot40404/creds/internal/vaultfiles"
)

func idInSlot(slot int) uuid.UUID {
	for {
		id := uuid.NewV7()
		if SlotOf(id) == slot {
			return id
		}
	}
}

func TestSlotSpread(t *testing.T) {
	var seen [slotCount]int
	for range 1000 {
		seen[SlotOf(uuid.NewV7())]++
	}
	for slot, n := range seen {
		if n == 0 {
			t.Fatalf("slot %d empty", slot)
		}
	}
}

func entryInSlot(slot byte, path string) Entry {
	e := sampleEntry()
	e.ID = idInSlot(int(slot) % slotCount)
	e.Path = path
	return e
}

func TestBucketRoundtrip(t *testing.T) {
	id, key := testutil.Keys(t)
	b := bucket{Format: bucketFormat, Slot: 3, Entries: []Entry{entryInSlot(3, "a"), entryInSlot(19, "b")}}
	data, err := sealBucket(id.Recipient(), key, b)
	if err != nil {
		t.Fatal(err)
	}
	got, err := openBucket(id, key, 3, data)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Entries) != 2 || got.Entries[1].Path != "b" {
		t.Fatalf("got %+v", got)
	}
}

func TestBucketWrongSlot(t *testing.T) {
	id, key := testutil.Keys(t)
	data, err := sealBucket(id.Recipient(), key, bucket{Format: bucketFormat, Slot: 3})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := openBucket(id, key, 4, data); !errors.Is(err, ErrBadBucket) {
		t.Fatalf("got %v", err)
	}
}

func TestBucketRejects(t *testing.T) {
	id, key := testutil.Keys(t)
	cases := map[string]bucket{
		"format":     {Format: 2, Slot: 1},
		"entry slot": {Format: bucketFormat, Slot: 1, Entries: []Entry{entryInSlot(2, "a")}},
		"invalid":    {Format: bucketFormat, Slot: 1, Entries: []Entry{entryInSlot(1, "")}},
	}
	for name, b := range cases {
		data, err := sealBucket(id.Recipient(), key, b)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := openBucket(id, key, 1, data); !errors.Is(err, ErrBadBucket) {
			t.Fatalf("%s: got %v", name, err)
		}
	}
}

func TestBucketForged(t *testing.T) {
	id, key := testutil.Keys(t)
	raw, err := json.Marshal(bucket{Format: bucketFormat, Slot: 0})
	if err != nil {
		t.Fatal(err)
	}
	cases := map[string]string{
		"missing mac": `{"bucket":` + string(raw) + `}`,
		"bad mac":     `{"mac":"00","bucket":` + string(raw) + `}`,
		"bad hex":     `{"mac":"zz","bucket":` + string(raw) + `}`,
		"no envelope": string(raw),
		"extra":       `{"mac":"","bucket":` + string(raw) + `,"x":1}`,
	}
	for name, pt := range cases {
		data, err := crypto.Seal(id.Recipient(), []byte(pt))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := openBucket(id, key, 0, data); !errors.Is(err, ErrBadBucket) {
			t.Fatalf("%s: got %v", name, err)
		}
	}
}

func TestBucketOtherKey(t *testing.T) {
	id, key := testutil.Keys(t)
	_, other := testutil.Keys(t)
	data, err := sealBucket(id.Recipient(), other, bucket{Format: bucketFormat, Slot: 0})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := openBucket(id, key, 0, data); !errors.Is(err, ErrBadBucket) {
		t.Fatalf("got %v", err)
	}
}

func TestBucketName(t *testing.T) {
	if vaultfiles.BucketName(10) != "0a.enc" || vaultfiles.BucketName(0) != "00.enc" {
		t.Fatal("bad name")
	}
}
