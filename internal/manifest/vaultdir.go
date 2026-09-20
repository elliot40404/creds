package manifest

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"

	"github.com/elliot40404/creds/internal/crypto"
	"github.com/elliot40404/creds/internal/fsutil"
	"github.com/elliot40404/creds/internal/vaultfiles"
)

func HashDir(dir string) (map[string]string, error) {
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, err
	}
	defer func() { _ = root.Close() }()
	out := map[string]string{}
	for _, p := range vaultfiles.Covered() {
		data, err := fsutil.ReadRegular(root, filepath.FromSlash(p), vaultfiles.MaxFileSize)
		if err != nil {
			return nil, fmt.Errorf("hash %s: %w", p, err)
		}
		sum := sha256.Sum256(data)
		out[p] = hex.EncodeToString(sum[:])
	}
	return out, nil
}

func Path(dir string) string {
	return filepath.Join(dir, vaultfiles.ManifestFile)
}

func Load(dir string, id *crypto.Identity) (Manifest, error) {
	data, err := fsutil.ReadFileLimit(Path(dir), MaxSize)
	if err != nil {
		return Manifest{}, err
	}
	return Open(id, data)
}

func Verify(dir string, id *crypto.Identity) (Manifest, error) {
	m, err := Load(dir, id)
	if err != nil {
		return m, err
	}
	have, err := HashDir(dir)
	if err != nil {
		return m, err
	}
	return m, Match(m, have)
}

func Match(m Manifest, have map[string]string) error {
	for _, f := range m.Files {
		if have[f.Path] != f.Hash {
			return fmt.Errorf("%w: %s does not match the manifest", ErrBadManifest, f.Path)
		}
	}
	return nil
}

func Write(dir string, id *crypto.Identity, parents []Manifest) (Manifest, error) {
	files, err := HashDir(dir)
	if err != nil {
		return Manifest{}, err
	}
	m, err := Next(parents, files)
	if err != nil {
		return m, err
	}
	data, err := Seal(id, m)
	if err != nil {
		return m, err
	}
	return m, fsutil.WriteFileAtomic(Path(dir), data)
}
