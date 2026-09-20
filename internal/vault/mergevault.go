package vault

import (
	"bytes"
	"fmt"
	"uuid"

	"github.com/elliot40404/creds/internal/crypto"
	"github.com/elliot40404/creds/internal/vaultfiles"
)

type Side int

const (
	Mine Side = iota + 1
	Theirs
)

type BucketSet map[string][]byte

type merger struct {
	id     *crypto.Identity
	rcpt   *crypto.Recipient
	key    []byte
	prefer map[uuid.UUID]Side
	left   int64
}

type slotResult struct {
	ours      []Entry
	theirs    []Entry
	merged    []Entry
	conflicts []Conflict
	data      []byte
}

func MergeVault(id *crypto.Identity, base, ours, theirs BucketSet, prefer map[uuid.UUID]Side) (BucketSet, []Conflict, error) {
	if err := checkNames(base, ours, theirs); err != nil {
		return nil, nil, err
	}
	key, err := id.MACKey()
	if err != nil {
		return nil, nil, err
	}
	defer clear(key)
	m := merger{id: id, rcpt: id.Recipient(), key: key, prefer: prefer, left: maxMergeTotal}
	out := BucketSet{}
	var allOurs, allTheirs, merged []Entry
	var conflicts []Conflict
	for slot := range slotCount {
		name := vaultfiles.BucketName(slot)
		r, err := m.slot(slot, base[name], ours[name], theirs[name])
		if err != nil {
			return nil, nil, fmt.Errorf("bucket %s: %w", name, err)
		}
		allOurs = append(allOurs, r.ours...)
		allTheirs = append(allTheirs, r.theirs...)
		merged = append(merged, r.merged...)
		conflicts = append(conflicts, r.conflicts...)
		if r.data != nil {
			out[name] = r.data
		}
	}
	conflicts = append(conflicts, pathConflicts(merged, allOurs, allTheirs)...)
	if len(conflicts) > 0 {
		return nil, conflicts, nil
	}
	return out, nil, nil
}

func checkNames(sets ...BucketSet) error {
	for _, s := range sets {
		for name := range s {
			if !vaultfiles.IsBucket(name) {
				return fmt.Errorf("%w: unknown bucket file %q", ErrBadBucket, name)
			}
		}
	}
	return nil
}

func (m *merger) slot(slot int, b, o, t []byte) (slotResult, error) {
	var r slotResult
	var err error
	if r.ours, err = m.open(slot, o); err != nil {
		return r, err
	}
	if bytes.Equal(o, t) {
		r.theirs, r.merged = r.ours, r.ours
		return r, nil
	}
	if r.theirs, err = m.open(slot, t); err != nil {
		return r, err
	}
	pinned := m.pinned(slot)
	switch {
	case bytes.Equal(b, t) && !pinned:
		r.merged = r.ours
		return r, nil
	case bytes.Equal(b, o) && !pinned:
		r.merged, r.data = r.theirs, t
		return r, nil
	}
	be, err := m.open(slot, b)
	if err != nil {
		return r, err
	}
	r.merged, r.conflicts = mergeByID(be, m.override(r.ours, r.theirs, Theirs), m.override(r.theirs, r.ours, Mine))
	if len(r.conflicts) > 0 {
		return r, nil
	}
	r.data, err = sealEntries(m.rcpt, m.key, slot, r.merged)
	return r, err
}

func (m *merger) open(slot int, data []byte) ([]Entry, error) {
	if data == nil {
		return nil, nil
	}
	if len(data) > vaultfiles.MaxFileSize || int64(len(data)) > m.left {
		return nil, errBucketTooLarge
	}
	m.left -= int64(len(data))
	b, err := openBucket(m.id, m.key, slot, data)
	return b.Entries, err
}

func (m *merger) pinned(slot int) bool {
	for id := range m.prefer {
		if SlotOf(id) == slot {
			return true
		}
	}
	return false
}

func (m *merger) override(dst, src []Entry, side Side) []Entry {
	out := make([]Entry, 0, len(dst))
	for _, e := range dst {
		if m.prefer[e.ID] != side {
			out = append(out, e)
		}
	}
	for _, e := range src {
		if m.prefer[e.ID] == side {
			out = append(out, e)
		}
	}
	return out
}
