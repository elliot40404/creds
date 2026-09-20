package crypto

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"

	"filippo.io/age"
	"golang.org/x/text/unicode/norm"
)

const ScryptLogN = 18

var (
	ErrWrongSecret = errors.New("wrong password or recovery code")
	ErrWrongKey    = errors.New("file is not encrypted to this vault's key, it may be tampered or from another vault")
)

func WrapIdentity(id *Identity, secret string, logN int) ([]byte, error) {
	return wrap(id, norm.NFC.String(secret), logN)
}

func wrap(id *Identity, secret string, logN int) ([]byte, error) {
	r, err := age.NewScryptRecipient(secret)
	if err != nil {
		return nil, fmt.Errorf("wrap identity: %w", err)
	}
	r.SetWorkFactor(logN)
	var buf bytes.Buffer
	w, err := age.Encrypt(&buf, r)
	if err != nil {
		return nil, fmt.Errorf("wrap identity: %w", err)
	}
	if _, err := io.WriteString(w, id.String()+"\n"); err != nil {
		return nil, fmt.Errorf("wrap identity: %w", err)
	}
	if err := w.Close(); err != nil {
		return nil, fmt.Errorf("wrap identity: %w", err)
	}
	return buf.Bytes(), nil
}

func UnwrapIdentity(data []byte, secret string) (*Identity, error) {
	if secret == "" {
		return nil, ErrWrongSecret
	}
	nfc := norm.NFC.String(secret)
	id, err := unwrap(data, nfc)
	if errors.Is(err, ErrWrongSecret) && nfc != secret {
		return unwrap(data, secret)
	}
	return id, err
}

func unwrap(data []byte, secret string) (*Identity, error) {
	sid, err := age.NewScryptIdentity(secret)
	if err != nil {
		return nil, fmt.Errorf("unwrap identity: %w", err)
	}
	sid.SetMaxWorkFactor(ScryptLogN)
	plain, err := decrypt(data, sid)
	if errors.Is(err, age.ErrIncorrectIdentity) {
		return nil, ErrWrongSecret
	}
	if err != nil {
		return nil, err
	}
	return ParseIdentity(strings.TrimSpace(string(plain)))
}

func decrypt(data []byte, id age.Identity) ([]byte, error) {
	r, err := age.Decrypt(bytes.NewReader(data), id)
	if err != nil {
		return nil, fmt.Errorf("decrypt: %w", err)
	}
	out, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("decrypt: %w", err)
	}
	return out, nil
}
