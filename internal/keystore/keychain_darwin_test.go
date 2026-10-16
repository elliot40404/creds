package keystore_test

import (
	"os"
	"testing"

	"github.com/elliot40404/creds/internal/keystore"
)

const wantMethod = "keychain"

func store(t testing.TB) keystore.Store {
	t.Helper()
	if os.Getenv("CREDS_TEST_KEYCHAIN") != "1" {
		t.Skip("set CREDS_TEST_KEYCHAIN=1 to use the login keychain")
	}
	s, err := keystore.Default()
	if err != nil {
		t.Fatal(err)
	}
	return s
}
