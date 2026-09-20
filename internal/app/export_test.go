package app

import (
	"encoding/csv"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/elliot40404/creds/internal/vault"
)

func richEntry(path string) vault.Entry {
	return vault.Entry{
		Path:     path,
		Name:     "Shop, \"prod\"",
		Type:     vault.TypeLogin,
		Username: "ops,team",
		Host:     "db.internal",
		URL:      "https://x.test/a?b=1,2",
		Notes:    "line one\nline \"two\"",
		Tags:     []string{"prod", "shop"},
		Fields: []vault.Field{
			{Name: "password", Value: "p,a\"ss\nword", Secret: true},
			{Name: "port", Value: "5432"},
		},
		Params: map[string]string{"sslmode": "require"},
	}
}

func exportVault(t *testing.T) (*Service, *fakePrompter) {
	t.Helper()
	s, fp, _, _ := initVault(t)
	for _, e := range []vault.Entry{richEntry("shop/web"), dbEntry("shop/db"), {Path: "misc/note", Type: vault.TypeNote, Notes: "hi"}} {
		if _, err := s.Add(e); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	fp.prompts = nil
	return s, fp
}

func TestExportCSV(t *testing.T) {
	t.Parallel()
	s, fp := exportVault(t)
	out := filepath.Join(t.TempDir(), "x.csv")
	fp.inputs = []string{ExportPhrase}
	n, err := s.ExportPlain(out, FormatCSV, false)
	if err != nil || n != 3 {
		t.Fatalf("export %d %v", n, err)
	}
	f, err := os.Open(filepath.Clean(out))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	rows, err := csv.NewReader(f).ReadAll()
	if err != nil {
		t.Fatalf("read csv: %v", err)
	}
	want := [][]string{
		csvHeader,
		{"misc/note", "note", "", "", "", "", "", ""},
		{"shop/db", "database", "", "db.internal", "", "password", entrySecret, "true"},
		{"shop/web", "login", "ops,team", "db.internal", "https://x.test/a?b=1,2", "password", "p,a\"ss\nword", "true"},
		{"shop/web", "login", "ops,team", "db.internal", "https://x.test/a?b=1,2", "port", "5432", "false"},
	}
	if len(rows) != len(want) {
		t.Fatalf("rows %d", len(rows))
	}
	for i := range want {
		if !slices.Equal(rows[i], want[i]) {
			t.Fatalf("row %d = %q want %q", i, rows[i], want[i])
		}
	}
}

func TestCSVCell(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"":             "",
		"plain":        "plain",
		"=1+1":         "'=1+1",
		"+cmd":         "'+cmd",
		"-2":           "'-2",
		"@SUM(A1)":     "'@SUM(A1)",
		"\tx":          "'\tx",
		"\rx":          "'\rx",
		"a=b":          "a=b",
		" =x":          " =x",
		"'quoted":      "'quoted",
		"\n=newline":   "\n=newline",
		"https://x.io": "https://x.io",
	}
	for in, want := range cases {
		if got := csvCell(in); got != want {
			t.Errorf("csvCell(%q) = %q want %q", in, got, want)
		}
	}
}

func TestCSVRowsEscape(t *testing.T) {
	t.Parallel()
	e := vault.Entry{Path: "=p", Type: vault.TypeLogin, Username: "@u", Host: "+h", URL: "-u", Fields: []vault.Field{{Name: "=n", Value: "=HYPERLINK(\"x\")", Secret: true}}}
	want := []string{"'=p", "login", "'@u", "'+h", "'-u", "'=n", "'=HYPERLINK(\"x\")", "true"}
	if got := csvRows(e); len(got) != 1 || !slices.Equal(got[0], want) {
		t.Fatalf("rows %q", got)
	}
}

func TestExportPhraseMismatch(t *testing.T) {
	t.Parallel()
	s, fp := exportVault(t)
	out := filepath.Join(t.TempDir(), "x.json")
	fp.inputs = []string{"yes"}
	_, err := s.ExportPlain(out, FormatJSON, false)
	if !errors.Is(err, ErrPhrase) {
		t.Fatalf("err %v", err)
	}
	if _, err := os.Lstat(out); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("file written: %v", err)
	}
}

func TestExportRefusesExisting(t *testing.T) {
	t.Parallel()
	s, fp := exportVault(t)
	out := filepath.Join(t.TempDir(), "x.json")
	if err := os.WriteFile(out, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ExportPlain(out, FormatJSON, false); !errors.Is(err, ErrFileExists) {
		t.Fatalf("err %v", err)
	}
	if len(fp.prompts) != 0 {
		t.Fatalf("prompted %v", fp.prompts)
	}
	fp.inputs = []string{ExportPhrase}
	if _, err := s.ExportPlain(out, FormatJSON, true); err != nil {
		t.Fatalf("overwrite: %v", err)
	}
	data, err := os.ReadFile(filepath.Clean(out))
	if err != nil || !strings.Contains(string(data), `"creds-export"`) {
		t.Fatalf("content %q %v", data, err)
	}
}

func TestExportUnknownFormat(t *testing.T) {
	t.Parallel()
	s, _ := exportVault(t)
	if _, err := s.ExportPlain(filepath.Join(t.TempDir(), "x"), "xml", false); !errors.Is(err, ErrExportFormat) {
		t.Fatalf("err %v", err)
	}
}

func TestExportCSVWarnsPartial(t *testing.T) {
	t.Parallel()
	s, fp := exportVault(t)
	fp.inputs = []string{ExportPhrase}
	if _, err := s.ExportPlain(filepath.Join(t.TempDir(), "x.csv"), FormatCSV, false); err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(fp.warned, csvNote) {
		t.Fatalf("warned = %v", fp.warned)
	}
	fp.warned, fp.inputs = nil, []string{ExportPhrase}
	if _, err := s.ExportPlain(filepath.Join(t.TempDir(), "x.json"), FormatJSON, false); err != nil {
		t.Fatal(err)
	}
	if len(fp.warned) != 0 {
		t.Fatalf("json warned = %v", fp.warned)
	}
}
