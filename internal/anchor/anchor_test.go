package anchor

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func tmp(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "anchor.json")
}

func TestSaveLoadRoundtrip(t *testing.T) {
	t.Parallel()
	path := tmp(t)
	want := Anchor{Manifest: strings.Repeat("a", 64), Generation: 7}
	if err := Save(path, want); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil || got != want {
		t.Fatalf("got %+v %v", got, err)
	}
}

func TestLoadIgnoresLegacyCommit(t *testing.T) {
	t.Parallel()
	path := tmp(t)
	if err := os.WriteFile(path, []byte(`{"manifest":"a","generation":3,"commit":"b"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil || got != (Anchor{Manifest: "a", Generation: 3}) {
		t.Fatalf("got %+v %v", got, err)
	}
}

func TestLoadMissing(t *testing.T) {
	t.Parallel()
	if _, err := Load(tmp(t)); !errors.Is(err, ErrNoAnchor) {
		t.Fatalf("err = %v", err)
	}
}

func TestLoadRejects(t *testing.T) {
	t.Parallel()
	for name, body := range map[string]string{
		"not json":             "{",
		"unknown member":       `{"manifest":"a","generation":1,"commit":"b","evil":1}`,
		"no manifest":          `{"manifest":"","generation":1,"commit":"b"}`,
		"zero generation":      `{"manifest":"a","generation":0,"commit":"b"}`,
		"generation as string": `{"manifest":"a","generation":"1","commit":"b"}`,
	} {
		t.Run(name, func(t *testing.T) {
			path := tmp(t)
			if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := Load(path); err == nil || errors.Is(err, ErrNoAnchor) {
				t.Fatalf("err = %v", err)
			}
		})
	}
}

func TestCheckRefusesOnlyOlder(t *testing.T) {
	t.Parallel()
	a := Anchor{Manifest: "x", Generation: 5}
	for _, gen := range []uint64{5, 6, 99} {
		if err := Check(a, gen); err != nil {
			t.Fatalf("generation %d refused: %v", gen, err)
		}
	}
	for _, gen := range []uint64{0, 1, 4} {
		if err := Check(a, gen); !errors.Is(err, ErrRollback) {
			t.Fatalf("generation %d accepted: %v", gen, err)
		}
	}
}
