package cli

import (
	"encoding/json/v2"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/elliot40404/creds/internal/app"
)

var evil = "x\x1b]52;c;cHduZWQ=\x07\x1b[2J" + string(rune(0x9b)) + "31m"

func noEscape(t *testing.T, what, s string) {
	t.Helper()
	if strings.ContainsAny(s, "\x1b\x07\r"+string(rune(0x9b))) {
		t.Fatalf("%s has raw control bytes: %q", what, s)
	}
}

func TestEscapedErrorOutput(t *testing.T) {
	t.Parallel()
	h := seeded(t)
	r := h.fail(&fake{}, "get", evil)
	noEscape(t, "get error", r.err)
	if !strings.Contains(r.err, `\x1b]52`) {
		t.Fatalf("err = %q", r.err)
	}
}

func TestEscapedEntryOutput(t *testing.T) {
	t.Parallel()
	h := seeded(t)
	file := filepath.Join(t.TempDir(), "x.json")
	h.ok(&fake{inputs: []string{app.ExportPhrase}}, "export", "--plain", "-o", file)
	writeEvilImport(t, file, func(e map[string]any) {
		e["username"], e["notes"], e["host"] = evil, evil+"\nmore", evil
	})
	h.ok(&fake{}, "import", "--overwrite", "--yes", file)
	for _, args := range [][]string{{"list"}, {"search", "web"}, {"get", "web/mail"}, {"get", "db/prod", "--show"}} {
		r := h.ok(&fake{}, args...)
		noEscape(t, strings.Join(args, " "), r.out+r.err)
		if !strings.Contains(r.out, `\x1b[2J`) {
			t.Fatalf("%v out = %q", args, r.out)
		}
	}
}

func TestImportRejectsControlNames(t *testing.T) {
	t.Parallel()
	for name, mutate := range map[string]func(map[string]any){
		"path":  func(e map[string]any) { e["path"] = evil },
		"name":  func(e map[string]any) { e["name"] = evil },
		"field": func(e map[string]any) { e["fields"].([]any)[0].(map[string]any)["name"] = evil },
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			h := seeded(t)
			file := filepath.Join(t.TempDir(), "x.json")
			h.ok(&fake{inputs: []string{app.ExportPhrase}}, "export", "--plain", "-o", file)
			writeEvilImport(t, file, mutate)
			r := h.fail(&fake{}, "import", "--overwrite", "--yes", file)
			noEscape(t, "import", r.out+r.err)
			if !strings.Contains(r.err, "control character") {
				t.Fatalf("err = %q", r.err)
			}
		})
	}
}

func writeEvilImport(t *testing.T, file string, mutate func(map[string]any)) {
	t.Helper()
	data, err := os.ReadFile(filepath.Clean(file))
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	for _, e := range doc["entries"].([]any) {
		if m := e.(map[string]any); len(m["fields"].([]any)) > 0 {
			mutate(m)
		}
	}
	if data, err = json.Marshal(doc); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestRawTextOnTerminal(t *testing.T) {
	t.Parallel()
	const v = "x\x1b]52;c;SGk=\x07y\n"
	if rawText(v, false) != v {
		t.Fatal("pipe output changed")
	}
	if got := rawText(v, true); strings.ContainsAny(got, "\x1b\x07") || !strings.HasSuffix(got, "y\n") {
		t.Fatalf("tty %q", got)
	}
}
