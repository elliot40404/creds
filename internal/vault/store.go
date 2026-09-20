package vault

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/elliot40404/creds/internal/crypto"
	"github.com/elliot40404/creds/internal/format"
	"github.com/elliot40404/creds/internal/fsutil"
	"github.com/elliot40404/creds/internal/vaultfiles"
)

const (
	maxMergeTotal = 256 << 20
)

var (
	ErrRecipientMismatch = errors.New("vault recipient does not match identity")
	errBucketTooLarge    = fmt.Errorf("bucket file %w", fsutil.ErrTooLarge)
)

var writeFile = fsutil.WriteFileAtomic

func Init(dir string, id *crypto.Identity) (*Vault, error) {
	v, err := newVault(dir, id)
	if err != nil {
		return nil, err
	}
	if err := v.create(); err != nil {
		v.Close()
		return nil, err
	}
	return v, nil
}

func (v *Vault) create() error {
	dir := v.dir
	metaPath := filepath.Join(dir, vaultfiles.MetaFile)
	if err := fsutil.CheckAbsent(metaPath); err != nil {
		return fmt.Errorf("init %s: %w", dir, err)
	}
	if err := fsutil.EnsureDir(filepath.Join(dir, vaultfiles.EntriesDir)); err != nil {
		return err
	}
	for i := range v.dirty {
		v.dirty[i] = true
	}
	if err := v.Save(); err != nil {
		return err
	}
	meta := format.VaultMeta{FormatVersion: format.CurrentVersion, Recipient: v.rcpt.String(), Created: v.now().UTC()}
	if err := format.SaveMeta(metaPath, meta); err != nil {
		return err
	}
	return nil
}

func Load(dir string, id *crypto.Identity) (*Vault, error) {
	v, err := newVault(dir, id)
	if err != nil {
		return nil, err
	}
	if err := v.open(); err != nil {
		v.Close()
		return nil, err
	}
	return v, nil
}

func (v *Vault) open() error {
	meta, err := format.LoadMeta(filepath.Join(v.dir, vaultfiles.MetaFile))
	if err != nil {
		return err
	}
	if meta.Recipient != v.rcpt.String() {
		return ErrRecipientMismatch
	}
	return v.loadBuckets()
}

func (v *Vault) loadBuckets() error {
	root, err := os.OpenRoot(filepath.Join(v.dir, vaultfiles.EntriesDir))
	if err != nil {
		return err
	}
	defer func() { _ = root.Close() }()
	for slot := range slotCount {
		if err := v.loadBucket(root, slot); err != nil {
			return fmt.Errorf("bucket %s: %w", vaultfiles.BucketName(slot), err)
		}
	}
	return nil
}

func (v *Vault) loadBucket(root *os.Root, slot int) error {
	data, err := fsutil.ReadRegular(root, vaultfiles.BucketName(slot), vaultfiles.MaxFileSize)
	if err != nil {
		return err
	}
	b, err := openBucket(v.id, v.key, slot, data)
	if err != nil {
		return err
	}
	for _, e := range b.Entries {
		if _, ok := v.entries[e.Path]; ok {
			return fmt.Errorf("%w: %s", ErrDuplicatePath, e.Path)
		}
		if _, ok := v.ids[e.ID]; ok {
			return fmt.Errorf("%w: %s", ErrDuplicateID, e.ID)
		}
		v.entries[e.Path] = e
		v.ids[e.ID] = e.Path
	}
	return nil
}

func (v *Vault) Save() error {
	slots := make([][]Entry, slotCount)
	for _, e := range v.entries {
		if s := SlotOf(e.ID); v.dirty[s] {
			slots[s] = append(slots[s], e)
		}
	}
	for slot, dirty := range v.dirty {
		if !dirty {
			continue
		}
		if err := v.saveBucket(slot, slots[slot]); err != nil {
			return err
		}
		v.dirty[slot] = false
	}
	return nil
}

func (v *Vault) saveBucket(slot int, entries []Entry) error {
	data, err := sealEntries(v.rcpt, v.key, slot, entries)
	if err != nil {
		return err
	}
	return writeFile(filepath.Join(v.dir, vaultfiles.EntriesDir, vaultfiles.BucketName(slot)), data)
}
