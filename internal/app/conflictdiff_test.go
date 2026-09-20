package app

import (
	"slices"
	"testing"

	"github.com/elliot40404/creds/internal/vault"
)

func TestDiffEntries(t *testing.T) {
	t.Parallel()
	ours := &vault.Entry{
		Path: "web/a", Type: vault.TypeLogin, Username: "bob", Params: map[string]string{"token": "T1"},
		Fields: []vault.Field{{Name: "password", Value: "MINE", Secret: true}, {Name: "region", Value: "eu"}},
	}
	theirs := &vault.Entry{
		Path: "web/a", Type: vault.TypeLogin, Username: "carol", Params: map[string]string{"token": "T2"},
		Fields: []vault.Field{{Name: "password", Value: "THEIRS", Secret: true}, {Name: "region", Value: "eu"}},
	}
	want := []FieldDiff{{Name: "password", Secret: true}, {Name: "token", Secret: true}, {Name: "username", Mine: "bob", Theirs: "carol"}}
	if got := diffEntries(ours, theirs); !slices.Equal(got, want) {
		t.Fatalf("got %+v", got)
	}
	if got := diffEntries(ours, nil); !slices.Equal(got, []FieldDiff{{Name: "entry", Mine: "kept", Theirs: deletedValue}}) {
		t.Fatalf("deleted %+v", got)
	}
}
