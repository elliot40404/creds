package app

import (
	"errors"
	"testing"

	"github.com/elliot40404/creds/internal/envfile"
	"github.com/elliot40404/creds/internal/vault"
)

func TestExtraFieldEnv(t *testing.T) {
	t.Parallel()
	x := ExtraField(vault.TypeEnv)
	if x.Label != "Variable name" || !x.Secret || x.Check == nil {
		t.Fatalf("env spec %+v", x)
	}
	for _, bad := range []string{"1X", "A B", "A-B", ""} {
		if err := x.Check(bad); !errors.Is(err, envfile.ErrBadKey) {
			t.Fatalf("%q: %v", bad, err)
		}
	}
	if err := x.Check("API_KEY"); err != nil {
		t.Fatal(err)
	}
	if g := ExtraField(vault.TypeLogin); g.Secret || g.Check != nil {
		t.Fatalf("login spec %+v", g)
	}
}
