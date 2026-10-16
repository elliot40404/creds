//go:build linux || darwin

package keystore_test

import (
	"errors"
	"testing"

	"github.com/elliot40404/creds/internal/keystore"
)

func TestForgetDropsKey(t *testing.T) {
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
	if _, err := s.Open(acct, sealed); !errors.Is(err, keystore.ErrOpen) {
		t.Fatalf("open after forget: %v", err)
	}
	again := seal(t, s, acct, []byte(identity))
	if got, err := s.Open(acct, again); err != nil || string(got) != identity {
		t.Fatalf("open after reseal: %v", err)
	}
	if _, err := s.Open(acct, sealed); !errors.Is(err, keystore.ErrOpen) {
		t.Fatalf("old blob with new key: %v", err)
	}
}
