package keystore

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
)

const (
	keySize     = 32
	blobVersion = 1
)

var (
	errNoKey   = errors.New("keystore: no key for this account")
	errKeySize = errors.New("keystore: stored key has the wrong size")
)

type keySource interface {
	key(name string, create bool) ([]byte, error)
	forget(name string) error
}

type aeadStore struct {
	method string
	src    keySource
}

func (s aeadStore) Method() string {
	return s.method
}

func (s aeadStore) Seal(account string, plain []byte) ([]byte, error) {
	ad, err := sealInput(account, plain)
	if err != nil {
		return nil, err
	}
	key, err := s.src.key(keyName(account), true)
	if err != nil {
		return nil, fmt.Errorf("keystore: %s seal: %w", s.method, err)
	}
	defer clear(key)
	gcm, err := newGCM(key)
	if err != nil {
		return nil, fmt.Errorf("keystore: %s seal: %w", s.method, err)
	}
	return gcm.Seal([]byte{blobVersion}, nil, plain, ad), nil
}

func (s aeadStore) Open(account string, sealed []byte) ([]byte, error) {
	ad, err := openInput(account, sealed)
	if err != nil {
		return nil, err
	}
	if sealed[0] != blobVersion {
		return nil, fmt.Errorf("%w: version %d", ErrOpen, sealed[0])
	}
	key, err := s.src.key(keyName(account), false)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrOpen, err)
	}
	defer clear(key)
	gcm, err := newGCM(key)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrOpen, err)
	}
	plain, err := gcm.Open(nil, nil, sealed[1:], ad)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrOpen, err)
	}
	return plain, nil
}

func (s aeadStore) Forget(account string) error {
	if _, err := entropy(account); err != nil {
		return err
	}
	if err := s.src.forget(keyName(account)); err != nil {
		return fmt.Errorf("keystore: %s forget: %w", s.method, err)
	}
	return nil
}

func newGCM(key []byte) (cipher.AEAD, error) {
	if len(key) != keySize {
		return nil, errKeySize
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCMWithRandomNonce(block)
}

func newKey() []byte {
	key := make([]byte, keySize)
	_, _ = rand.Read(key)
	return key
}

func keyName(account string) string {
	sum := sha256.Sum256([]byte(account))
	return hex.EncodeToString(sum[:])
}
