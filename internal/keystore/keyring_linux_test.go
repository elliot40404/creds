package keystore_test

import (
	"errors"
	"testing"

	"github.com/elliot40404/creds/internal/keystore"
)

const wantMethod = "keyring"

func store(t testing.TB) keystore.Store {
	t.Helper()
	s, err := keystore.Default()
	if errors.Is(err, keystore.ErrUnavailable) {
		t.Skip(err)
	}
	if err != nil {
		t.Fatal(err)
	}
	return s
}
