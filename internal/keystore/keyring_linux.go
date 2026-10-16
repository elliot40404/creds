package keystore

import (
	"errors"
	"fmt"
	"os"
	"strconv"

	"golang.org/x/sys/unix"
)

const (
	methodKeyring = "keyring"
	keyringType   = "user"
	keyringPrefix = "creds:"
)

type keyring struct{}

var errNotPossessed = errors.New("keystore: user keyring is not linked to the session keyring")

func platform() (Store, error) {
	if err := keyringReady(); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrUnavailable, err)
	}
	return aeadStore{method: methodKeyring, src: keyring{}}, nil
}

func keyringReady() error {
	user, err := unix.KeyctlGetKeyringID(unix.KEY_SPEC_USER_KEYRING, true)
	if err != nil {
		return err
	}
	found, err := unix.KeyctlSearch(unix.KEY_SPEC_SESSION_KEYRING, "keyring", "_uid."+strconv.Itoa(os.Getuid()), 0)
	if errors.Is(err, unix.ENOKEY) || err == nil && found != user {
		return errNotPossessed
	}
	return err
}

func (keyring) key(name string, create bool) ([]byte, error) {
	desc := keyringPrefix + name
	id, err := unix.KeyctlSearch(unix.KEY_SPEC_USER_KEYRING, keyringType, desc, 0)
	if gone(err) && create {
		id, err = addKeyring(desc)
	}
	if gone(err) {
		return nil, errNoKey
	}
	if err != nil {
		return nil, err
	}
	return readKeyring(id)
}

func (keyring) forget(name string) error {
	id, err := unix.KeyctlSearch(unix.KEY_SPEC_USER_KEYRING, keyringType, keyringPrefix+name, 0)
	if gone(err) {
		return nil
	}
	if err != nil {
		return err
	}
	_, err = unix.KeyctlInt(unix.KEYCTL_INVALIDATE, id, 0, 0, 0)
	return err
}

func gone(err error) bool {
	return errors.Is(err, unix.ENOKEY) || errors.Is(err, unix.EKEYREVOKED) || errors.Is(err, unix.EKEYEXPIRED)
}

func addKeyring(desc string) (int, error) {
	key := newKey()
	defer clear(key)
	return unix.AddKey(keyringType, desc, key, unix.KEY_SPEC_USER_KEYRING)
}

func readKeyring(id int) ([]byte, error) {
	buf := make([]byte, keySize+1)
	n, err := unix.KeyctlBuffer(unix.KEYCTL_READ, id, buf, 0)
	if err == nil && n != keySize {
		err = errKeySize
	}
	if err != nil {
		clear(buf)
		return nil, err
	}
	return buf[:keySize], nil
}
