package format

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/elliot40404/creds/internal/fsutil"
	"github.com/elliot40404/creds/internal/vaultfiles"
)

func writeFile(t *testing.T, path, data string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := fsutil.WriteFileAtomic(path, []byte(data)); err != nil {
		t.Fatal(err)
	}
}

func TestMetaRoundtrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), vaultfiles.MetaFile)
	want := VaultMeta{FormatVersion: CurrentVersion, Recipient: "age1x", Created: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)}
	if err := SaveMeta(path, want); err != nil {
		t.Fatal(err)
	}
	got, err := LoadMeta(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.FormatVersion != want.FormatVersion || got.Recipient != want.Recipient || !got.Created.Equal(want.Created) {
		t.Fatalf("got %+v want %+v", got, want)
	}
}

func TestLoadMetaRejects(t *testing.T) {
	cases := map[string]struct {
		data string
		err  error
	}{
		"unknown field": {`{"format_version":2,"recipient":"a","created":"2026-01-02T03:04:05Z","extra":1}`, nil},
		"duplicate key": {`{"format_version":2,"recipient":"a","recipient":"b","created":"2026-01-02T03:04:05Z"}`, nil},
		"not json":      {`nope`, nil},
		"newer":         {`{"format_version":4,"future":true}`, ErrUnknownVersion},
		"older":         {`{"recipient":"a"}`, ErrOldVersion},
	}
	for name, c := range cases {
		path := filepath.Join(t.TempDir(), vaultfiles.MetaFile)
		writeFile(t, path, c.data)
		_, err := LoadMeta(path)
		if err == nil {
			t.Fatalf("%s: want error", name)
		}
		if c.err != nil && !errors.Is(err, c.err) {
			t.Fatalf("%s: got %v want %v", name, err, c.err)
		}
	}
}

func TestLoadMetaMissing(t *testing.T) {
	_, err := LoadMeta(filepath.Join(t.TempDir(), vaultfiles.MetaFile))
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("got %v", err)
	}
}

func TestLoadMetaRefusesOversizedAndNonRegular(t *testing.T) {
	dir := t.TempDir()
	big := filepath.Join(dir, "big.json")
	writeFile(t, big, `{"format_version":1,"recipient":"`+strings.Repeat("a", maxMetaSize)+`"}`)
	if _, err := LoadMeta(big); err == nil {
		t.Fatal("oversized meta accepted")
	}
	sub := filepath.Join(dir, "sub")
	if err := os.Mkdir(sub, 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadMeta(sub); err == nil {
		t.Fatal("directory accepted")
	}
}

func TestParseMetaRefusesTooLarge(t *testing.T) {
	t.Parallel()
	data := append([]byte(`{"format_version":1}`), []byte(strings.Repeat(" ", 2<<20))...)
	if _, err := ParseMeta(data); !errors.Is(err, errMetaTooLarge) {
		t.Fatalf("want errMetaTooLarge, got %v", err)
	}
}
