//go:build !windows && !linux

package keystore_test

import (
	"errors"
	"testing"

	"github.com/elliot40404/creds/internal/keystore"
)

func TestDefaultUnavailable(t *testing.T) {
	t.Parallel()
	s, err := keystore.Default()
	if s != nil || !errors.Is(err, keystore.ErrUnavailable) {
		t.Fatalf("got %v, %v", s, err)
	}
}
