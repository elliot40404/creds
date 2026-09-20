package cli

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/elliot40404/creds/internal/app"
)

func TestExportPlain(t *testing.T) {
	t.Parallel()
	h := seeded(t)
	dir := t.TempDir()
	for _, format := range []string{"json", "csv"} {
		out := filepath.Join(dir, "x."+format)
		r := h.ok(&fake{inputs: []string{app.ExportPhrase}}, "export", "--plain", "--format", format, "-o", out)
		if r.out != "" || r.err != "exported 2 entries to "+out+"\n" {
			t.Fatalf("out %q err %q", r.out, r.err)
		}
		noLeak(t, r, loginHidden, dbHidden)
		checkPrivate(t, out)
		data, err := os.ReadFile(filepath.Clean(out))
		if err != nil || !strings.Contains(string(data), loginHidden) {
			t.Fatalf("%s missing secret: %v", format, err)
		}
		h.fail(&fake{}, "export", "--plain", "--format", format, "-o", out)
		h.fail(&fake{}, "export", "--plain", "--format", format, "-o", out, "--yes")
		h.fail(&fake{}, "export", "--plain", "--format", format, "-o", out, "--yes", "--confirm-plaintext", "yes")
		h.ok(&fake{}, "export", "--plain", "--format", format, "-o", out, "--yes", "--confirm-plaintext", app.ExportPhrase)
	}
}

func TestExportPlainRefused(t *testing.T) {
	t.Parallel()
	h := seeded(t)
	out := filepath.Join(t.TempDir(), "x.json")
	r := h.fail(&fake{inputs: []string{"yes"}}, "export", "--plain", "-o", out)
	if !strings.Contains(r.err, "confirmation phrase") {
		t.Fatalf("err %q", r.err)
	}
	h.fail(&fake{}, "export", "-o", out)
	h.fail(&fake{}, "export", "--plain")
	h.fail(&fake{}, "export", "--plain", "--format", "xml", "-o", out)
	if _, err := os.Lstat(out); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("file written: %v", err)
	}
}

func TestExportEncrypted(t *testing.T) {
	t.Parallel()
	h := seeded(t)
	out := filepath.Join(t.TempDir(), "backup.tar")
	r := h.ok(&fake{}, "export", "--encrypted", "-o", out)
	if !strings.HasPrefix(r.err, "exported ") || !strings.HasSuffix(r.err, " files to "+out+"\n") {
		t.Fatalf("err %q", r.err)
	}
	checkPrivate(t, out)
	data, err := os.ReadFile(filepath.Clean(out))
	if err != nil || strings.Contains(string(data), loginHidden) || !strings.Contains(string(data), "vault.json") {
		t.Fatalf("tar content: %v", err)
	}
	h.fail(&fake{}, "export", "--encrypted", "-o", out)
	h.fail(&fake{}, "export", "--encrypted", "--plain", "-o", out)
	h.fail(&fake{}, "export", "--encrypted", "--format", "csv", "-o", out)
	h.ok(&fake{}, "export", "--encrypted", "-o", out, "-y")
}

func TestImport(t *testing.T) {
	t.Parallel()
	h := seeded(t)
	out := filepath.Join(t.TempDir(), "x.json")
	h.ok(&fake{inputs: []string{app.ExportPhrase}}, "export", "--plain", "-o", out)
	want := withoutStamps(h.ok(&fake{}, "get", "--show", "db/prod").out)

	r := h.ok(&fake{}, "import", out)
	if r.err != "skipped db/prod: path exists, use --overwrite\nskipped web/mail: path exists, use --overwrite\nimported 0, replaced 0, skipped 2\n" {
		t.Fatalf("err %q", r.err)
	}
	r = h.ok(&fake{}, "import", out, "--overwrite", "--yes")
	if r.err != "imported 0, replaced 2, skipped 0\n" {
		t.Fatalf("err %q", r.err)
	}

	h2 := newHarness(t)
	h2.init()
	r = h2.ok(&fake{}, "import", out)
	if r.err != "imported 2, replaced 0, skipped 0\n" {
		t.Fatalf("err %q", r.err)
	}
	noLeak(t, r, loginHidden, dbHidden)
	if got := withoutStamps(h2.ok(&fake{}, "get", "--show", "db/prod").out); got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	h2.fail(&fake{}, "import", filepath.Join(t.TempDir(), "missing.json"))
	h2.fail(&fake{}, "import", writeFile(t, t.TempDir(), "bad.json", `{"format":"x","version":1,"entries":[]}`))
}
