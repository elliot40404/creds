package format

import (
	"encoding/json/v2"
	"errors"
	"fmt"
	"time"

	"github.com/elliot40404/creds/internal/fsutil"
)

const (
	CurrentVersion = 3
	maxMetaSize    = 1 << 20
)

var (
	ErrUnknownVersion = errors.New("vault format is newer than this program")
	ErrOldVersion     = errors.New("vault format needs migration")
	errMetaTooLarge   = fmt.Errorf("vault meta %w", fsutil.ErrTooLarge)
)

type VaultMeta struct {
	FormatVersion int       `json:"format_version"`
	Recipient     string    `json:"recipient"`
	Created       time.Time `json:"created"`
}

type versionProbe struct {
	FormatVersion int `json:"format_version"`
}

func LoadMeta(path string) (VaultMeta, error) {
	data, err := fsutil.ReadFileLimit(path, maxMetaSize)
	if err != nil {
		return VaultMeta{}, err
	}
	return ParseMeta(data)
}

func ParseMeta(data []byte) (VaultMeta, error) {
	var m VaultMeta
	if len(data) > maxMetaSize {
		return m, errMetaTooLarge
	}
	v, err := probeVersion(data)
	if err != nil {
		return m, err
	}
	if err := checkVersion(v, CurrentVersion); err != nil {
		return m, err
	}
	if err := json.Unmarshal(data, &m, json.RejectUnknownMembers(true)); err != nil {
		return m, fmt.Errorf("decode meta: %w", err)
	}
	return m, nil
}

func SaveMeta(path string, m VaultMeta) error {
	return fsutil.WriteJSON(path, m)
}

func readMeta(path string) (int, error) {
	data, err := fsutil.ReadFileLimit(path, maxMetaSize)
	if err != nil {
		return 0, err
	}
	return probeVersion(data)
}

func probeVersion(data []byte) (int, error) {
	var p versionProbe
	if err := json.Unmarshal(data, &p); err != nil {
		return 0, fmt.Errorf("decode meta version: %w", err)
	}
	return p.FormatVersion, nil
}

func checkVersion(v, current int) error {
	switch {
	case v > current:
		return fmt.Errorf("%w: %d", ErrUnknownVersion, v)
	case v < current:
		return fmt.Errorf("%w: %d", ErrOldVersion, v)
	}
	return nil
}
