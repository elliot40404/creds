package keystore

import "errors"

const (
	MaxPlain   = 1 << 16
	maxSealed  = 2 * MaxPlain
	maxAccount = 256
)

var (
	ErrUnavailable = errors.New("keystore: no os key store on this system")
	ErrOpen        = errors.New("keystore: sealed data could not be opened")
	errAccount     = errors.New("keystore: account must be 1 to 256 bytes")
	errTooLarge    = errors.New("keystore: data too large")
)

type Store interface {
	Method() string
	Seal(account string, plain []byte) ([]byte, error)
	Open(account string, sealed []byte) ([]byte, error)
	Forget(account string) error
}

func Default() (Store, error) {
	return platform()
}

func entropy(account string) ([]byte, error) {
	if account == "" || len(account) > maxAccount {
		return nil, errAccount
	}
	return []byte("creds keystore v1\x00" + account), nil
}

func sealInput(account string, plain []byte) ([]byte, error) {
	if len(plain) > MaxPlain {
		return nil, errTooLarge
	}
	return entropy(account)
}

func openInput(account string, sealed []byte) ([]byte, error) {
	salt, err := entropy(account)
	if err != nil {
		return nil, err
	}
	if len(sealed) == 0 || len(sealed) > maxSealed {
		return nil, ErrOpen
	}
	return salt, nil
}
