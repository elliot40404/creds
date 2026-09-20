package manifest

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json/v2"
	"errors"
	"fmt"
	"math"
	"slices"
	"strings"

	"github.com/elliot40404/creds/internal/vaultfiles"
)

const (
	envelopeName = "manifest"
	Format       = 1
	MaxSize      = 1 << 20
	MaxDepth     = 2
	hashLen      = sha256.Size * 2
)

var (
	ErrBadManifest = errors.New("bad vault manifest")
	ErrOverflow    = errors.New("vault manifest generation overflow")
)

type File struct {
	Path string `json:"path"`
	Hash string `json:"hash"`
}

type Manifest struct {
	Format     int      `json:"format"`
	Generation uint64   `json:"generation"`
	Parents    []string `json:"parents"`
	Files      []File   `json:"files"`
}

func New(generation uint64, parents []string, files map[string]string) (Manifest, error) {
	m := Manifest{Format: Format, Generation: generation, Parents: canonicalParents(parents), Files: canonicalFiles(files)}
	return m, m.check()
}

func Next(parents []Manifest, files map[string]string) (Manifest, error) {
	var gen uint64
	ids := make([]string, 0, len(parents))
	for _, p := range parents {
		if p.Generation > gen {
			gen = p.Generation
		}
		id, err := p.ID()
		if err != nil {
			return Manifest{}, err
		}
		ids = append(ids, id)
	}
	if gen == math.MaxUint64 {
		return Manifest{}, ErrOverflow
	}
	return New(gen+1, ids, files)
}

func canonicalParents(parents []string) []string {
	if len(parents) == 0 {
		return []string{}
	}
	out := slices.Clone(parents)
	slices.Sort(out)
	return slices.Compact(out)
}

func canonicalFiles(files map[string]string) []File {
	out := make([]File, 0, len(files))
	for p, h := range files {
		out = append(out, File{Path: p, Hash: h})
	}
	slices.SortFunc(out, func(a, b File) int { return strings.Compare(a.Path, b.Path) })
	return out
}

func (m Manifest) check() error {
	switch {
	case m.Format != Format:
		return fmt.Errorf("%w: format %d", ErrBadManifest, m.Format)
	case m.Generation == 0:
		return fmt.Errorf("%w: generation 0", ErrBadManifest)
	case len(m.Parents) > MaxDepth:
		return fmt.Errorf("%w: %d parents", ErrBadManifest, len(m.Parents))
	}
	if err := checkParents(m.Parents); err != nil {
		return err
	}
	return checkFiles(m.Files)
}

func checkParents(parents []string) error {
	for i, p := range parents {
		if !isHash(p) {
			return fmt.Errorf("%w: parent %q", ErrBadManifest, p)
		}
		if i > 0 && parents[i-1] >= p {
			return fmt.Errorf("%w: parents not sorted or repeated", ErrBadManifest)
		}
	}
	return nil
}

func checkFiles(files []File) error {
	want := vaultfiles.Covered()
	if len(files) != len(want) {
		return fmt.Errorf("%w: %d files, want %d", ErrBadManifest, len(files), len(want))
	}
	slices.Sort(want)
	for i, f := range files {
		if f.Path != want[i] {
			return fmt.Errorf("%w: file %q, want %q", ErrBadManifest, f.Path, want[i])
		}
		if !isHash(f.Hash) {
			return fmt.Errorf("%w: hash for %s", ErrBadManifest, f.Path)
		}
	}
	return nil
}

func isHash(s string) bool {
	if len(s) != hashLen {
		return false
	}
	_, err := hex.DecodeString(s)
	return err == nil && s == strings.ToLower(s)
}

func (m Manifest) payload() ([]byte, error) {
	if err := m.check(); err != nil {
		return nil, err
	}
	data, err := json.Marshal(m)
	if err != nil {
		return nil, fmt.Errorf("encode manifest: %w", err)
	}
	return data, nil
}

func (m Manifest) ID() (string, error) {
	data, err := m.payload()
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

func (m Manifest) FileHash(path string) string {
	for _, f := range m.Files {
		if f.Path == path {
			return f.Hash
		}
	}
	return ""
}

func parsePayload(raw []byte) (Manifest, error) {
	var m Manifest
	if err := json.Unmarshal(raw, &m, json.RejectUnknownMembers(true)); err != nil {
		return m, fmt.Errorf("%w: %w", ErrBadManifest, err)
	}
	if m.Parents == nil {
		m.Parents = []string{}
	}
	if err := m.check(); err != nil {
		return m, err
	}
	return m, checkCanonical(m, raw)
}

func checkCanonical(m Manifest, raw []byte) error {
	again, err := m.payload()
	if err != nil {
		return err
	}
	if string(again) != string(raw) {
		return fmt.Errorf("%w: not canonical", ErrBadManifest)
	}
	return nil
}
