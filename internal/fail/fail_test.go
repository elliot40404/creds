package fail

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/elliot40404/creds/internal/app"
	"github.com/elliot40404/creds/internal/gitsync"
	"github.com/elliot40404/creds/internal/session"
	"github.com/elliot40404/creds/internal/vault"
)

func TestClassify(t *testing.T) {
	custom := errors.New("custom")
	tests := []struct {
		name  string
		err   error
		extra []Rule
		code  Code
		msg   string
		hint  string
	}{
		{"locked", session.ErrNoSession, nil, Locked, "vault is locked", hintUnlock},
		{"expired", fmt.Errorf("load: %w", session.ErrExpired), nil, Locked, "session expired", hintUnlock},
		{"no vault", app.ErrNoVault, nil, NoVault, "vault not initialized", hintInit},
		{"conflict", gitsync.ErrConflict, nil, Conflict, "merge conflict", hintStatus},
		{"not found keeps context", fmt.Errorf("db/prod: %w", vault.ErrNotFound), nil, NotFound, "db/prod: entry not found", hintSearch},
		{"no remote", gitsync.ErrNoRemote, nil, General, "no remote configured", "run creds remote add <url>"},
		{"unknown", custom, nil, General, "custom", ""},
		{"extra rule", custom, []Rule{{Target: custom, Code: Usage, Hint: "pass flags instead of -i"}}, Usage, "custom", "pass flags instead of -i"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := Classify(tt.err, tt.extra...)
			if e.Code != tt.code || e.Msg != tt.msg || e.Hint != tt.hint {
				t.Fatalf("got %d %q %q", e.Code, e.Msg, e.Hint)
			}
			if !errors.Is(e, tt.err) {
				t.Fatal("unwrap lost original")
			}
		})
	}
}

func TestClassifyPassthrough(t *testing.T) {
	if Classify(nil) != nil {
		t.Fatal("nil err must give nil")
	}
	in := &Error{Msg: "x", Code: Usage}
	if Classify(fmt.Errorf("wrap: %w", in)) != in {
		t.Fatal("existing Error must pass through")
	}
}

func TestHintsNameACommand(t *testing.T) {
	for _, r := range rules {
		if r.Hint == "" || strings.Contains(r.Hint, "creds") || strings.HasPrefix(r.Hint, hintRerun) {
			continue
		}
		t.Errorf("%v: hint %q names no command", r.Target, r.Hint)
	}
}
