package envfile

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func writeProject(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ProjectFile), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestParseRef(t *testing.T) {
	good := map[string]Ref{
		"prod/db:user":     {Path: "prod/db", Field: "user"},
		"prod/db|url":      {Path: "prod/db", Format: "url"},
		"my app/api:token": {Path: "my app/api", Field: "token"},
		"a:b":              {Path: "a", Field: "b"},
	}
	for in, want := range good {
		got, err := ParseRef(in)
		if err != nil || got != want {
			t.Errorf("%q: got %+v, %v", in, got, err)
		}
	}
	bad := []string{"", "path", ":field", "path:", "|url", "path|", "a:b|c", "a|b:c", "a:b:c", " a:b", "a: b", "a:b ", "a\x1bX:b", "a:b\u009b"}
	for _, in := range bad {
		if _, err := ParseRef(in); !errors.Is(err, ErrBadRef) {
			t.Errorf("%q: got %v", in, err)
		}
	}
}

func TestLoadProject(t *testing.T) {
	dir := writeProject(t, `env = "work/app-env"

[map]
DATABASE_URL = "work/db|url"
DB_USER = "work/db:username"
`)
	got, err := LoadProject(dir)
	if err != nil {
		t.Fatal(err)
	}
	want := Project{
		Env: "work/app-env",
		Map: map[string]Ref{
			"DATABASE_URL": {Path: "work/db", Format: "url"},
			"DB_USER":      {Path: "work/db", Field: "username"},
		},
		File: filepath.Join(dir, ProjectFile),
		Sum:  got.Sum,
	}
	if !reflect.DeepEqual(got, want) || len(got.Sum) != 64 || !filepath.IsAbs(got.File) {
		t.Fatalf("got %+v", got)
	}
	wantRefs := []string{"all fields of work/app-env", "DATABASE_URL = work/db|url", "DB_USER = work/db:username"}
	if refs := got.Refs(); !reflect.DeepEqual(refs, wantRefs) {
		t.Fatalf("refs %q", refs)
	}
}

func TestLoadProjectPartial(t *testing.T) {
	got, err := LoadProject(writeProject(t, `env = "x"`))
	if err != nil || got.Env != "x" || got.Map == nil || len(got.Map) != 0 {
		t.Fatalf("env only: %+v, %v", got, err)
	}
	got, err = LoadProject(writeProject(t, "[map]\nA = \"x:y\"\n"))
	if err != nil || got.Env != "" || len(got.Map) != 1 {
		t.Fatalf("map only: %+v, %v", got, err)
	}
}

func TestLoadProjectMissing(t *testing.T) {
	if _, err := LoadProject(t.TempDir()); !errors.Is(err, ErrNoProject) {
		t.Fatalf("got %v", err)
	}
}

func TestLoadProjectCurrentDirOnly(t *testing.T) {
	parent := writeProject(t, `env = "x"`)
	child := filepath.Join(parent, "sub")
	if err := os.Mkdir(child, 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadProject(child); !errors.Is(err, ErrNoProject) {
		t.Fatalf("got %v", err)
	}
}

func TestLoadProjectErrors(t *testing.T) {
	cases := []struct {
		body string
		want error
		msg  string
	}{
		{"", ErrEmptyProject, ""},
		{"[map]\n", ErrEmptyProject, ""},
		{`env = "x"` + "\nextra = 1\n", nil, "unknown key extra"},
		{"[map]\nA = \"x:y\"\n[other]\nb = 1\n", nil, "unknown key other"},
		{"[map]\nA = \"nofield\"\n", ErrBadRef, "map.A"},
		{"[map]\n\"1A\" = \"x:y\"\n", ErrBadKey, "map"},
		{"[map]\nA = 1\n", nil, "map"},
		{`env = ""`, nil, "env"},
		{`env = " x"`, nil, "env"},
		{`env = 3`, nil, "env"},
		{"env = \"x\"\nenv = \"y\"\n", nil, "env"},
		{"not toml", nil, ""},
	}
	for _, c := range cases {
		_, err := LoadProject(writeProject(t, c.body))
		if err == nil {
			t.Errorf("%q: no error", c.body)
			continue
		}
		if c.want != nil && !errors.Is(err, c.want) {
			t.Errorf("%q: got %v want %v", c.body, err, c.want)
		}
		if !strings.Contains(err.Error(), c.msg) || !strings.Contains(err.Error(), ProjectFile) {
			t.Errorf("%q: message %q missing %q", c.body, err, c.msg)
		}
	}
}

func TestLoadProjectTooLarge(t *testing.T) {
	body := `env = "x"` + "\n#" + strings.Repeat("x", maxSize)
	if _, err := LoadProject(writeProject(t, body)); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("got %v", err)
	}
}
