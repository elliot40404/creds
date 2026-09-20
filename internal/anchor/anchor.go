package anchor

import (
	"errors"
	"fmt"
	"io/fs"

	"github.com/elliot40404/creds/internal/fsutil"
	"github.com/elliot40404/creds/internal/manifest"
)

const maxSize = 1 << 16

var (
	ErrNoAnchor = errors.New("no trusted vault state recorded yet")
	ErrRollback = errors.New("vault state is older than the one this machine already trusted")
	errCorrupt  = errors.New("trusted vault state file is not valid")
)

type Anchor struct {
	Manifest   string `json:"manifest"`
	Generation uint64 `json:"generation"`
}

type record struct {
	Anchor `json:",inline"`
	Commit string `json:"commit,omitzero"`
}

func Load(path string) (Anchor, error) {
	var r record
	err := fsutil.ReadJSON(path, maxSize, &r)
	if errors.Is(err, fs.ErrNotExist) {
		return Anchor{}, ErrNoAnchor
	}
	if err != nil {
		return Anchor{}, fmt.Errorf("%w: %w", errCorrupt, err)
	}
	if r.Manifest == "" || r.Generation == 0 {
		return Anchor{}, errCorrupt
	}
	return r.Anchor, nil
}

func Save(path string, a Anchor) error {
	return fsutil.WriteJSON(path, a)
}

func Record(path string, m manifest.Manifest) error {
	id, err := m.ID()
	if err != nil {
		return err
	}
	return Save(path, Anchor{Manifest: id, Generation: m.Generation})
}

func Check(a Anchor, generation uint64) error {
	if generation < a.Generation {
		return fmt.Errorf("%w: generation %d, trusted %d", ErrRollback, generation, a.Generation)
	}
	return nil
}
