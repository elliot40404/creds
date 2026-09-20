package app

import (
	"errors"
	"maps"
	"slices"
	"testing"

	"github.com/elliot40404/creds/internal/render"
	"github.com/elliot40404/creds/internal/vault"
)

func TestTypesCoverAll(t *testing.T) {
	t.Parallel()
	for _, ty := range Types() {
		if !ty.Valid() {
			t.Fatalf("invalid type %q", ty)
		}
	}
	if len(TypeNames()) != len(Types()) {
		t.Fatal("names and types differ")
	}
	Types()[0] = "x"
	if Types()[0] != vault.TypeLogin {
		t.Fatal("Types leaked internal slice")
	}
}

func TestFieldSpecs(t *testing.T) {
	t.Parallel()
	login := FieldSpecs(vault.TypeLogin)
	if !slices.Equal(login, []FieldSpec{{Key: FieldUsername}, {Key: FieldPassword, Secret: true}, {Key: FieldURL}}) {
		t.Fatalf("login %+v", login)
	}
	login[0].Key = "x"
	if FieldSpecs(vault.TypeLogin)[0].Key != FieldUsername {
		t.Fatal("FieldSpecs leaked internal slice")
	}
	if len(FieldSpecs(vault.TypeGeneric)) != 0 {
		t.Fatal("generic has specs")
	}
	if !IsSpecField(vault.TypeAPI, "key") || IsSpecField(vault.TypeAPI, FieldPassword) {
		t.Fatal("IsSpecField")
	}
}

func TestSetField(t *testing.T) {
	t.Parallel()
	var e vault.Entry
	SetField(&e, FieldHost, "h", false)
	SetField(&e, "pin", "1", true)
	SetField(&e, "empty", "", true)
	if e.Host != "h" || len(e.Fields) != 1 || e.Fields[0] != (vault.Field{Name: "pin", Value: "1", Secret: true}) {
		t.Fatalf("entry %+v", e)
	}
	SetField(&e, "pin", "2", false)
	if e.Fields[0].Value != "2" || !e.Fields[0].Secret {
		t.Fatalf("update %+v", e.Fields)
	}
	SetField(&e, "pin", "", false)
	if len(e.Fields) != 0 {
		t.Fatalf("delete %+v", e.Fields)
	}
}

func TestSetConn(t *testing.T) {
	t.Parallel()
	e := vault.Entry{Type: vault.TypeDatabase}
	if err := SetConn(&e, render.Postgres, "postgres://bob:pw@db:5433/app?sslmode=require"); err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"engine": render.Postgres, "scheme": "postgres", "port": "5433", "database": "app", FieldPassword: "pw"}
	for k, v := range want {
		if got, _ := Lookup(e, k); got != v {
			t.Errorf("%s = %q, want %q", k, got, v)
		}
	}
	if e.Host != "db" || e.Username != "bob" || e.Params["sslmode"] != "require" {
		t.Fatalf("entry %+v", e)
	}
	if f := e.Fields[slices.IndexFunc(e.Fields, func(f vault.Field) bool { return f.Name == FieldPassword })]; !f.Secret {
		t.Fatal("password not secret")
	}
	if err := SetConn(&e, render.Redis, "http://x"); err == nil {
		t.Fatal("want parse error")
	}
	if !slices.Equal(Engines(), []string{render.Postgres, render.Redis, render.Mongo}) {
		t.Fatal("engines")
	}
}

func TestDefaultField(t *testing.T) {
	t.Parallel()
	pw := vault.Field{Name: FieldPassword, Value: "p", Secret: true}
	tok := vault.Field{Name: FieldToken, Value: "t", Secret: true}
	pin := vault.Field{Name: "pin", Value: "1", Secret: true}
	cmd := vault.Field{Name: FieldCommand, Value: "make"}
	cases := []struct {
		e    vault.Entry
		want string
	}{
		{vault.Entry{Fields: []vault.Field{pin, tok, pw}}, FieldPassword},
		{vault.Entry{Fields: []vault.Field{pin, tok}}, FieldToken},
		{vault.Entry{Fields: []vault.Field{cmd, pin}}, "pin"},
		{vault.Entry{Type: vault.TypeCommand, Fields: []vault.Field{cmd}}, FieldCommand},
	}
	for _, c := range cases {
		if got, err := DefaultField(c.e); err != nil || got != c.want {
			t.Fatalf("got %q %v, want %q", got, err, c.want)
		}
	}
	if _, err := DefaultField(vault.Entry{Fields: []vault.Field{cmd}}); !errors.Is(err, ErrNoSecret) {
		t.Fatalf("err %v", err)
	}
}

func TestSetConnSecretParams(t *testing.T) {
	t.Parallel()
	e := vault.Entry{Type: vault.TypeDatabase, Fields: []vault.Field{{Name: "token", Value: "old"}}}
	if err := SetConn(&e, render.Postgres, "postgres://bob@db/app?password=QS3CRET&sslpassword=K3Y&sslmode=require"); err != nil {
		t.Fatal(err)
	}
	if !maps.Equal(e.Params, map[string]string{"sslmode": "require"}) {
		t.Fatalf("params %v", e.Params)
	}
	want := []vault.Field{{Name: FieldPassword, Value: "QS3CRET", Secret: true}, {Name: "sslpassword", Value: "K3Y", Secret: true}}
	got := slices.DeleteFunc(slices.Clone(e.Fields), func(f vault.Field) bool { return !f.Secret && f.Name != "token" })
	if !slices.Equal(got, want) {
		t.Fatalf("fields %+v", e.Fields)
	}
	_, c, err := databaseConn(e)
	if err != nil || c.Password != "QS3CRET" || c.Params["sslpassword"] != "K3Y" {
		t.Fatalf("conn %+v %v", c, err)
	}
	if err := SetConn(&e, render.Postgres, "postgres://bob@db/app"); err != nil {
		t.Fatal(err)
	}
	if slices.ContainsFunc(e.Fields, func(f vault.Field) bool { return f.Secret }) {
		t.Fatalf("stale secrets %+v", e.Fields)
	}
}
