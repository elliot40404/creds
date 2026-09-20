package vault

import (
	"errors"
	"fmt"
	"slices"
	"time"
	"uuid"

	"github.com/elliot40404/creds/internal/crypto"
	"github.com/elliot40404/creds/internal/vaultfiles"
)

var (
	ErrNotFound      = errors.New("entry not found")
	ErrDuplicatePath = errors.New("duplicate entry path")
	ErrDuplicateID   = errors.New("duplicate entry id")
)

type Vault struct {
	dir     string
	id      *crypto.Identity
	rcpt    *crypto.Recipient
	key     []byte
	entries map[string]Entry
	ids     map[uuid.UUID]string
	dirty   [slotCount]bool
	now     func() time.Time
	history int
	machine string
	gone    map[uuid.UUID]Entry
}

func (v *Vault) Close() {
	clear(v.key)
}

func newVault(dir string, id *crypto.Identity) (*Vault, error) {
	key, err := id.MACKey()
	if err != nil {
		return nil, err
	}
	return &Vault{
		dir:     dir,
		id:      id,
		rcpt:    id.Recipient(),
		key:     key,
		entries: map[string]Entry{},
		ids:     map[uuid.UUID]string{},
		now:     time.Now,
		history: vaultfiles.DefaultHistory,
		gone:    map[uuid.UUID]Entry{},
	}, nil
}

func (v *Vault) Identity() *crypto.Identity { return v.id }

func (v *Vault) Get(path string) (Entry, error) {
	e, ok := v.entries[path]
	if !ok {
		return Entry{}, fmt.Errorf("%w: %s", ErrNotFound, path)
	}
	return e.Clone(), nil
}

func (v *Vault) List() []Entry {
	out := make([]Entry, 0, len(v.entries))
	for _, e := range v.entries {
		out = append(out, e.Clone())
	}
	slices.SortFunc(out, comparePath)
	return out
}

func (v *Vault) Put(e Entry) (Entry, error) {
	if err := e.Validate(); err != nil {
		return Entry{}, err
	}
	now := v.now().UTC()
	e.Created = e.Created.UTC()
	if e.ID == uuid.Nil() {
		e.ID = uuid.NewV7()
		e.Created = now
	}
	if err := v.checkUnique(e); err != nil {
		return Entry{}, err
	}
	if e.Created.IsZero() {
		e.Created = now
	}
	e.History = v.nextHistory(e)
	e.Updated = now
	e.Machine = v.machine
	v.add(e.Clone())
	return e, nil
}

func (v *Vault) Delete(path string) error {
	e, ok := v.entries[path]
	if !ok {
		return fmt.Errorf("%w: %s", ErrNotFound, path)
	}
	delete(v.entries, path)
	delete(v.ids, e.ID)
	v.gone[e.ID] = e
	v.dirty[SlotOf(e.ID)] = true
	return nil
}

func (v *Vault) checkUnique(e Entry) error {
	if old, ok := v.entries[e.Path]; ok && old.ID != e.ID {
		return fmt.Errorf("%w: %s", ErrDuplicatePath, e.Path)
	}
	if p, ok := v.ids[e.ID]; ok && p != e.Path {
		return fmt.Errorf("%w: %s", ErrDuplicateID, e.ID)
	}
	return nil
}

func (v *Vault) add(e Entry) {
	v.entries[e.Path] = e
	v.ids[e.ID] = e.Path
	v.dirty[SlotOf(e.ID)] = true
}

func (v *Vault) SetHistory(limit int, machine string) {
	v.history, v.machine = max(limit, 0), machine
}

func (v *Vault) nextHistory(e Entry) []Revision {
	if v.history <= 0 {
		return nil
	}
	prev, ok := v.entries[e.Path]
	if !ok {
		prev, ok = v.gone[e.ID]
	}
	if !ok {
		return mergeHistory(e.History, nil, v.history)
	}
	if sameValues(prev.revision(), e.revision()) {
		return mergeHistory(e.History, prev.History, v.history)
	}
	return mergeHistory([]Revision{prev.revision()}, mergeHistory(e.History, prev.History, v.history), v.history)
}
