package tui

import (
	"testing"

	"github.com/elliot40404/creds/internal/vault"
)

func TestDetailMasksSecretParams(t *testing.T) {
	t.Parallel()
	e := vault.Entry{Params: map[string]string{"password": "p", "sslmode": "require"}}
	for _, f := range fieldsOf(e) {
		if f.secret != (f.name == "password") {
			t.Fatalf("field %+v", f)
		}
	}
}
