package manifest

import (
	"fmt"

	"github.com/elliot40404/creds/internal/crypto"
	"github.com/elliot40404/creds/internal/fsutil"
)

func Seal(id *crypto.Identity, m Manifest) ([]byte, error) {
	raw, err := m.payload()
	if err != nil {
		return nil, err
	}
	key, err := id.ManifestMACKey()
	if err != nil {
		return nil, err
	}
	pt, err := crypto.Sign(key, envelopeName, raw)
	if err != nil {
		return nil, err
	}
	return crypto.Seal(id.Recipient(), pt)
}

func Open(id *crypto.Identity, data []byte) (Manifest, error) {
	if len(data) > MaxSize {
		return Manifest{}, fmt.Errorf("vault manifest %w", fsutil.ErrTooLarge)
	}
	pt, err := crypto.Open(id, data)
	if err != nil {
		return Manifest{}, err
	}
	key, err := id.ManifestMACKey()
	if err != nil {
		return Manifest{}, err
	}
	raw, err := crypto.Verify(key, envelopeName, pt)
	if err != nil {
		return Manifest{}, fmt.Errorf("%w: %w", ErrBadManifest, err)
	}
	return parsePayload(raw)
}
