package keystore

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"time"
)

const (
	methodKeychain  = "keychain"
	securityPath    = "/usr/bin/security"
	keychainService = "creds-session"
	keychainTimeout = 5 * time.Second
	exitNotFound    = 44
)

type keychain struct{}

func platform() (Store, error) {
	if _, err := os.Stat(securityPath); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrUnavailable, err)
	}
	return aeadStore{method: methodKeychain, src: keychain{}}, nil
}

func (keychain) key(name string, create bool) ([]byte, error) {
	key, err := findKeychain(name)
	if errors.Is(err, errNoKey) && create {
		if err = addKeychain(name); err == nil {
			key, err = findKeychain(name)
		}
	}
	return key, err
}

func (keychain) forget(name string) error {
	_, err := security(nil, "delete-generic-password", "-s", keychainService, "-a", name)
	if errors.Is(err, errNoKey) {
		return nil
	}
	return err
}

func findKeychain(name string) ([]byte, error) {
	out, err := security(nil, "find-generic-password", "-s", keychainService, "-a", name, "-w")
	defer clear(out)
	if err != nil {
		return nil, err
	}
	text := bytes.TrimSpace(out)
	if len(text) != hex.EncodedLen(keySize) {
		return nil, errKeySize
	}
	key := make([]byte, keySize)
	if _, err := hex.Decode(key, text); err != nil {
		clear(key)
		return nil, errKeySize
	}
	return key, nil
}

func addKeychain(name string) error {
	key := newKey()
	defer clear(key)
	line := make([]byte, 0, 256)
	line = append(line, "add-generic-password -U -s "+keychainService+" -a "+name+" -w "...)
	line = hex.AppendEncode(line, key)
	line = append(line, '\n')
	defer clear(line)
	out, err := security(line, "-i")
	clear(out)
	return err
}

func security(stdin []byte, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), keychainTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, securityPath, args...)
	if stdin != nil {
		cmd.Stdin = bytes.NewReader(stdin)
	}
	out, err := cmd.Output()
	var exit *exec.ExitError
	if errors.As(err, &exit) && exit.ExitCode() == exitNotFound {
		return out, errNoKey
	}
	if err != nil {
		return out, fmt.Errorf("security %s: %w", args[0], err)
	}
	return out, nil
}
