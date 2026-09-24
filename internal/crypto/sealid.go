package crypto

import (
	"errors"
	"strings"

	"filippo.io/age"
)

func SealIdentity(id *Identity, r age.Recipient) ([]byte, error) {
	return encryptIdentity(id, r)
}

func OpenIdentity(data []byte, key age.Identity) (*Identity, error) {
	if !IsAgeFile(data) {
		return nil, errors.New("open identity: not an age file")
	}
	plain, err := decrypt(data, key)
	if errors.Is(err, age.ErrIncorrectIdentity) {
		return nil, ErrWrongKey
	}
	if err != nil {
		return nil, err
	}
	return ParseIdentity(strings.TrimSpace(string(plain)))
}
