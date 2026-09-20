package app

import (
	"encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"time"
	"uuid"

	"github.com/elliot40404/creds/internal/vault"
)

const importLimit = 64 << 20

var ErrImportFile = errors.New("invalid import file")

type ImportResult struct {
	Added    []string
	Replaced []string
	Skipped  []string
}

func (s *Service) Import(r io.Reader, overwrite bool) (ImportResult, error) {
	entries, err := decodeImport(r)
	if err != nil {
		return ImportResult{}, err
	}
	var res ImportResult
	_, err = s.write(func(v *vault.Vault) (vault.Entry, error) {
		res = ImportResult{}
		for _, e := range entries {
			if err := importOne(v, e, overwrite, &res); err != nil {
				return vault.Entry{}, err
			}
		}
		return vault.Entry{}, nil
	})
	if err != nil {
		return ImportResult{}, err
	}
	return res, nil
}

func importOne(v *vault.Vault, e vault.Entry, overwrite bool, res *ImportResult) error {
	e.ID, e.Created, e.Updated = uuid.Nil(), time.Time{}, time.Time{}
	old, err := v.Get(e.Path)
	switch {
	case errors.Is(err, vault.ErrNotFound):
		res.Added = append(res.Added, e.Path)
	case err != nil:
		return err
	case !overwrite:
		res.Skipped = append(res.Skipped, e.Path)
		return nil
	default:
		e.ID, e.Created = old.ID, old.Created
		res.Replaced = append(res.Replaced, e.Path)
	}
	_, err = v.Put(e)
	return err
}

func decodeImport(r io.Reader) ([]vault.Entry, error) {
	data, err := io.ReadAll(io.LimitReader(r, importLimit+1))
	if err != nil {
		return nil, err
	}
	if len(data) > importLimit {
		return nil, fmt.Errorf("%w: too large", ErrImportFile)
	}
	var f exportFile
	if err := json.Unmarshal(data, &f, json.RejectUnknownMembers(true)); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrImportFile, err)
	}
	if f.Format != exportFormat || f.Version != exportVersion {
		return nil, fmt.Errorf("%w: unsupported format %q version %d", ErrImportFile, f.Format, f.Version)
	}
	return f.Entries, nil
}
