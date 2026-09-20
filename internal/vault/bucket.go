package vault

import (
	"crypto/sha256"
	"encoding/json/v2"
	"errors"
	"fmt"
	"slices"
	"uuid"

	"github.com/elliot40404/creds/internal/crypto"
	"github.com/elliot40404/creds/internal/vaultfiles"
)

const (
	envelopeName = "bucket"
	bucketFormat = 1
	slotCount    = vaultfiles.BucketCount
)

var ErrBadBucket = errors.New("bad bucket")

type bucket struct {
	Format  int     `json:"format"`
	Slot    int     `json:"slot"`
	Entries []Entry `json:"entries"`
}

func SlotOf(id uuid.UUID) int {
	sum := sha256.Sum256(id[:])
	return int(sum[0]) % slotCount
}

func sealEntries(r *crypto.Recipient, key []byte, slot int, entries []Entry) ([]byte, error) {
	sorted := slices.SortedFunc(slices.Values(entries), comparePath)
	if sorted == nil {
		sorted = []Entry{}
	}
	return sealBucket(r, key, bucket{Format: bucketFormat, Slot: slot, Entries: sorted})
}

func sealBucket(r *crypto.Recipient, key []byte, b bucket) ([]byte, error) {
	raw, err := json.Marshal(b)
	if err != nil {
		return nil, fmt.Errorf("encode bucket: %w", err)
	}
	pt, err := crypto.Sign(key, envelopeName, raw)
	if err != nil {
		return nil, err
	}
	return crypto.Seal(r, pt)
}

func openBucket(id *crypto.Identity, key []byte, slot int, data []byte) (bucket, error) {
	var b bucket
	pt, err := crypto.Open(id, data)
	if err != nil {
		return b, err
	}
	raw, err := crypto.Verify(key, envelopeName, pt)
	if err != nil {
		return b, fmt.Errorf("%w: %w", ErrBadBucket, err)
	}
	if err := json.Unmarshal(raw, &b, json.RejectUnknownMembers(true)); err != nil {
		return b, fmt.Errorf("%w: %w", ErrBadBucket, err)
	}
	return b, checkBucket(b, slot)
}

func checkBucket(b bucket, slot int) error {
	switch {
	case b.Format != bucketFormat:
		return fmt.Errorf("%w: unknown format %d", ErrBadBucket, b.Format)
	case b.Slot != slot:
		return fmt.Errorf("%w: slot %d in file for slot %d", ErrBadBucket, b.Slot, slot)
	}
	for _, e := range b.Entries {
		if SlotOf(e.ID) != slot {
			return fmt.Errorf("%w: entry %s in wrong slot", ErrBadBucket, e.ID)
		}
		if err := e.Validate(); err != nil {
			return fmt.Errorf("%w: %w", ErrBadBucket, err)
		}
	}
	return nil
}
