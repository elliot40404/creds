package vault

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
	"testing"

	"github.com/elliot40404/creds/internal/crypto"
	"github.com/elliot40404/creds/internal/testutil"
)

const fuzzSlot = 3

func validBucketJSON(f *testing.F) []byte {
	e := sampleEntry()
	e.ID = idInSlot(fuzzSlot)
	raw, err := json.Marshal(bucket{Format: bucketFormat, Slot: fuzzSlot, Entries: []Entry{e}})
	if err != nil {
		f.Fatal(err)
	}
	return raw
}

func addJunkSeeds(f *testing.F, valid []byte) {
	f.Add(valid)
	for _, s := range []string{
		"",
		"{}",
		"null",
		"[]",
		"\x00\xff",
		`{"mac":"zz","bucket":{}}`,
		`{"mac":"","bucket":null}`,
		`{"format":1,"slot":3,"entries":null}`,
		`{"format":1,"slot":3,"entries":[{"path":"../x\u0000","type":"login"}]}`,
		`{"format":1,"slot":3,"entries":[{"id":"not-a-uuid"}]}`,
		`{"format":1,"slot":3,"entries":[],"extra":"=cmd"}`,
	} {
		f.Add([]byte(s))
	}
}

func FuzzOpenBucketPlaintext(f *testing.F) {
	id, key := testutil.Keys(f)
	raw := validBucketJSON(f)
	env, err := crypto.Sign(key, envelopeName, raw)
	if err != nil {
		f.Fatal(err)
	}
	addJunkSeeds(f, env)
	f.Fuzz(func(t *testing.T, pt []byte) {
		data, err := crypto.Seal(id.Recipient(), pt)
		if err != nil {
			t.Fatal(err)
		}
		checkOpened(t, id, key, data)
	})
}

func FuzzOpenBucketJSON(f *testing.F) {
	id, key := testutil.Keys(f)
	addJunkSeeds(f, validBucketJSON(f))
	f.Fuzz(func(t *testing.T, raw []byte) {
		if !jsontext.Value(raw).IsValid() {
			return
		}
		pt, err := crypto.Sign(key, envelopeName, raw)
		if err != nil {
			t.Fatal(err)
		}
		data, err := crypto.Seal(id.Recipient(), pt)
		if err != nil {
			t.Fatal(err)
		}
		checkOpened(t, id, key, data)
	})
}

func checkOpened(t *testing.T, id *crypto.Identity, key, data []byte) {
	t.Helper()
	b, err := openBucket(id, key, fuzzSlot, data)
	if err != nil {
		return
	}
	again, err := sealBucket(id.Recipient(), key, b)
	if err != nil {
		t.Fatalf("reseal: %v", err)
	}
	if _, err := openBucket(id, key, fuzzSlot, again); err != nil {
		t.Fatalf("reopen: %v", err)
	}
}
