package crypto

import (
	"bytes"
	"errors"
	"fmt"

	"filippo.io/age"
)

const minPadSize = 4096

var ageHeader = []byte("age-encryption.org/v1\n")

func Pad(plaintext []byte) []byte {
	size := minPadSize
	for size < len(plaintext) {
		size <<= 1
	}
	out := make([]byte, size)
	n := copy(out, plaintext)
	for i := n; i < size; i++ {
		out[i] = ' '
	}
	return out
}

func Seal(r *Recipient, plaintext []byte) ([]byte, error) {
	padded := Pad(plaintext)
	var buf bytes.Buffer
	buf.Grow(len(padded) + 2048)
	w, err := age.Encrypt(&buf, r.r)
	if err != nil {
		return nil, fmt.Errorf("seal: %w", err)
	}
	if _, err := w.Write(padded); err != nil {
		return nil, fmt.Errorf("seal: %w", err)
	}
	if err := w.Close(); err != nil {
		return nil, fmt.Errorf("seal: %w", err)
	}
	return buf.Bytes(), nil
}

func Open(id *Identity, data []byte) ([]byte, error) {
	if !IsAgeFile(data) {
		return nil, errors.New("open: not an age file")
	}
	out, err := decrypt(data, id.id)
	if errors.Is(err, age.ErrIncorrectIdentity) {
		return nil, ErrWrongKey
	}
	return out, err
}

func IsAgeFile(data []byte) bool {
	return bytes.HasPrefix(data, ageHeader)
}
