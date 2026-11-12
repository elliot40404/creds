package testutil

import (
	"bytes"
	"crypto/sha256"
	"runtime"
	"slices"
	"sync"

	"github.com/elliot40404/creds/internal/keystore"
)

type FakeKeys struct {
	mu      sync.Mutex
	SealErr error
	seals   int
	forgot  []string
}

func Keystore() (keystore.Store, error) {
	if runtime.GOOS == "windows" {
		return keystore.Default()
	}
	return &FakeKeys{}, nil
}

func NoKeystore() (keystore.Store, error) {
	return nil, keystore.ErrUnavailable
}

func (k *FakeKeys) Method() string { return "fake" }

func (k *FakeKeys) Seal(account string, plain []byte) ([]byte, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.seals++
	if k.SealErr != nil {
		return nil, k.SealErr
	}
	return append(keyTag(account), mask(plain)...), nil
}

func (k *FakeKeys) Open(account string, sealed []byte) ([]byte, error) {
	tag := keyTag(account)
	if len(sealed) < len(tag) || !bytes.Equal(sealed[:len(tag)], tag) {
		return nil, keystore.ErrOpen
	}
	return mask(sealed[len(tag):]), nil
}

func (k *FakeKeys) Forget(account string) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.forgot = append(k.forgot, account)
	return nil
}

func (k *FakeKeys) Seals() int {
	k.mu.Lock()
	defer k.mu.Unlock()
	return k.seals
}

func (k *FakeKeys) Forgotten() []string {
	k.mu.Lock()
	defer k.mu.Unlock()
	return slices.Clone(k.forgot)
}

func keyTag(account string) []byte {
	sum := sha256.Sum256([]byte(account))
	return sum[:8]
}

func mask(b []byte) []byte {
	out := make([]byte, len(b))
	for i, c := range b {
		out[i] = c ^ 0x5a
	}
	return out
}
