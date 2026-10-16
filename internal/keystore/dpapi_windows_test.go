package keystore_test

import (
	"testing"

	"github.com/elliot40404/creds/internal/keystore"
)

const wantMethod = "dpapi"

func store(t testing.TB) keystore.Store {
	t.Helper()
	s, err := keystore.Default()
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestForgetKeepsBlobUsable(t *testing.T) {
	t.Parallel()
	s := store(t)
	acct := account(t, s)
	sealed := seal(t, s, acct, []byte(identity))
	if err := s.Forget(acct); err != nil {
		t.Fatal(err)
	}
	if err := s.Forget(acct); err != nil {
		t.Fatal(err)
	}
	if got, err := s.Open(acct, sealed); err != nil || string(got) != identity {
		t.Fatalf("open after forget: %v", err)
	}
}
