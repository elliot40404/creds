package envfile

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func trusted(t *testing.T, store string, p Project) bool {
	t.Helper()
	ok, err := Trusted(store, p)
	if err != nil {
		t.Fatal(err)
	}
	return ok
}

func TestTrust(t *testing.T) {
	store := filepath.Join(t.TempDir(), "trust.json")
	dir := writeProject(t, `env = "a"`)
	p, err := LoadProject(dir)
	if err != nil {
		t.Fatal(err)
	}
	if trusted(t, store, p) {
		t.Fatal("trusted before trust")
	}
	if err := Trust(store, p); err != nil {
		t.Fatal(err)
	}
	if !trusted(t, store, p) {
		t.Fatal("not trusted after trust")
	}
	if fi, err := os.Stat(store); err != nil || (runtime.GOOS != "windows" && fi.Mode().Perm() != 0o600) {
		t.Fatalf("store %v %v", fi, err)
	}
	other := writeProject(t, `env = "a"`)
	if q, _ := LoadProject(other); trusted(t, store, q) {
		t.Fatal("same content in other dir trusted")
	}
	if err := os.WriteFile(filepath.Join(dir, ProjectFile), []byte(`env = "b"`), 0o600); err != nil {
		t.Fatal(err)
	}
	if q, _ := LoadProject(dir); trusted(t, store, q) {
		t.Fatal("changed file trusted")
	}
	if trusted(t, store, Project{File: p.File}) {
		t.Fatal("empty sum trusted")
	}
}

func TestTrustBadStore(t *testing.T) {
	for _, body := range []string{"", "{}", `{"version":2,"projects":{}}`, `{"version":1,"projects":{},"x":1}`, "not json"} {
		store := filepath.Join(t.TempDir(), "trust.json")
		if err := os.WriteFile(store, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := Trusted(store, Project{}); !errors.Is(err, ErrTrustFile) {
			t.Errorf("%q: got %v", body, err)
		}
		if err := Trust(store, Project{}); !errors.Is(err, ErrTrustFile) {
			t.Errorf("%q: trust got %v", body, err)
		}
	}
	store := filepath.Join(t.TempDir(), "trust.json")
	if err := os.WriteFile(store, []byte(`{"version":1}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Trust(store, Project{File: "f", Sum: "s"}); err != nil {
		t.Fatal(err)
	}
	if !trusted(t, store, Project{File: "f", Sum: "s"}) {
		t.Fatal("not trusted")
	}
}

func TestTrustRejectsNonUTF8Path(t *testing.T) {
	store := filepath.Join(t.TempDir(), "trust.json")
	p := Project{File: "/tmp/\xff/" + ProjectFile, Sum: "abc"}
	if err := Trust(store, p); err == nil {
		t.Fatal("want error")
	}
	if _, err := os.Stat(store); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("store written: %v", err)
	}
}
