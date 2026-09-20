package format

import (
	"encoding/json/v2"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/elliot40404/creds/internal/vaultfiles"
)

type upgrade func(t *txn) error

var upgrades = []upgrade{upgradeV0, upgradeV1, upgradeV2}

func Migrate(dir string) (from int, err error) {
	if err := restore(dir); err != nil {
		return 0, fmt.Errorf("restore backup: %w", err)
	}
	from, err = readMeta(filepath.Join(dir, vaultfiles.MetaFile))
	if err != nil {
		return 0, err
	}
	if err := checkVersion(from, CurrentVersion); !errors.Is(err, ErrOldVersion) {
		return from, err
	}
	return from, runChain(dir, from)
}

func Pending(dir string) (bool, error) {
	_, err := os.Lstat(filepath.Join(dir, backupDir))
	if err == nil {
		return true, nil
	}
	if !errors.Is(err, fs.ErrNotExist) {
		return false, err
	}
	v, err := readMeta(filepath.Join(dir, vaultfiles.MetaFile))
	return v != CurrentVersion, err
}

func runChain(dir string, from int) error {
	t := newTx(dir)
	for v, up := range upgrades[from:] {
		if err := up(t); err != nil {
			return errors.Join(fmt.Errorf("upgrade from version %d: %w", from+v, err), t.rollback())
		}
	}
	return t.commit()
}

type metaV0 struct {
	Recipient string    `json:"recipient"`
	CreatedAt time.Time `json:"created_at"`
}

func upgradeV0(t *txn) error {
	data, err := t.read(vaultfiles.MetaFile)
	if err != nil {
		return err
	}
	var old metaV0
	if err := json.Unmarshal(data, &old, json.RejectUnknownMembers(true)); err != nil {
		return fmt.Errorf("decode v0 meta: %w", err)
	}
	out, err := json.Marshal(VaultMeta{FormatVersion: 1, Recipient: old.Recipient, Created: old.CreatedAt})
	if err != nil {
		return err
	}
	return t.write(vaultfiles.MetaFile, out)
}

func upgradeV1(t *txn) error {
	return bumpVersion(t, 2)
}

func upgradeV2(t *txn) error {
	if err := t.write(vaultfiles.IgnoreFile, []byte(vaultfiles.IgnoreRules)); err != nil {
		return err
	}
	return bumpVersion(t, 3)
}

func bumpVersion(t *txn, to int) error {
	data, err := t.read(vaultfiles.MetaFile)
	if err != nil {
		return err
	}
	var m VaultMeta
	if err := json.Unmarshal(data, &m, json.RejectUnknownMembers(true)); err != nil {
		return fmt.Errorf("decode v%d meta: %w", to-1, err)
	}
	m.FormatVersion = to
	out, err := json.Marshal(m)
	if err != nil {
		return err
	}
	return t.write(vaultfiles.MetaFile, out)
}
