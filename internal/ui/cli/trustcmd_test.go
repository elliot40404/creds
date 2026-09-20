package cli

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestTrustFlow(t *testing.T) {
	h := imported(t)
	dir := t.TempDir()
	file := filepath.Join(dir, ".creds.toml")
	writeFile(t, dir, ".creds.toml", "[map]\nDB = \"db/prod|url\"\n")
	t.Chdir(dir)
	r := h.fail(&fake{}, "env")
	if !strings.Contains(r.err, "not trusted") || !strings.Contains(r.err, "run creds trust") || strings.Contains(r.out, dbConn) {
		t.Fatalf("untrusted: %q %q", r.out, r.err)
	}
	if r = h.fail(&fake{confirms: []bool{false}}, "env"); !strings.Contains(r.err, "aborted") {
		t.Fatalf("declined: %q", r.err)
	}
	r = h.ok(&fake{confirms: []bool{true}}, "trust")
	if r.out != "DB = db/prod|url\n" || !strings.Contains(r.err, "trusted "+file) {
		t.Fatalf("trust: %q %q", r.out, r.err)
	}
	if r = h.ok(&fake{}, "env"); !strings.Contains(r.out, dbConn) {
		t.Fatalf("trusted env: %q", r.out)
	}
	writeFile(t, dir, ".creds.toml", "[map]\nDB = \"db/prod|url\"\nX = \"proj/env:API_KEY\"\n")
	h.fail(&fake{}, "env")
	t.Chdir(t.TempDir())
	h.fail(&fake{}, "trust", "--yes")
	h.ok(&fake{}, "trust", "--yes", dir)
	t.Chdir(dir)
	if r = h.ok(&fake{}, "env"); !strings.Contains(r.out, "X="+envHidden) {
		t.Fatalf("retrusted env: %q", r.out)
	}
}

func TestTerminalNotInteractive(t *testing.T) {
	t.Parallel()
	term, _ := pipeTerminal(t, "y\n")
	if term.Interactive() {
		t.Fatal("pipe is interactive")
	}
}

func TestTerminalEscapesOutput(t *testing.T) {
	t.Parallel()
	term, out := ttyPipe(t, "y\n")
	if err := term.Show("x\x1b]52;c;eA==\x07", "a\x1b[2J"); err != nil {
		t.Fatal(err)
	}
	if _, err := term.Confirm("trust \x1b]0;t\x07?"); err != nil {
		t.Fatal(err)
	}
	if strings.ContainsAny(out.String(), "\x1b\x07") {
		t.Fatalf("out = %q", out.String())
	}
}

func TestTrustListAndRemove(t *testing.T) {
	h := imported(t)
	dir := t.TempDir()
	file := filepath.Join(dir, ".creds.toml")
	writeFile(t, dir, ".creds.toml", "[map]\nDB = \"db/prod|url\"\n")
	t.Chdir(dir)
	if r := h.ok(&fake{}, "trust", "list"); r.out != "" {
		t.Fatalf("empty list: %q", r.out)
	}
	h.ok(&fake{confirms: []bool{true}}, "trust")
	if r := h.ok(&fake{}, "trust", "list"); strings.TrimSpace(r.out) != file {
		t.Fatalf("list: %q", r.out)
	}
	r := h.ok(&fake{confirms: []bool{true}}, "trust", "remove", file)
	if !strings.Contains(r.err, "forgot "+file) {
		t.Fatalf("remove: %q", r.err)
	}
	if r = h.ok(&fake{}, "trust", "list"); r.out != "" {
		t.Fatalf("list after remove: %q", r.out)
	}
	if r = h.fail(&fake{}, "trust", "remove", "--yes", file); !strings.Contains(r.err, "not trusted") {
		t.Fatalf("second remove: %q", r.err)
	}
	if r = h.fail(&fake{}, "env"); !strings.Contains(r.err, "run creds trust") {
		t.Fatalf("env after remove: %q", r.err)
	}
}

func TestTrustListJSON(t *testing.T) {
	h := imported(t)
	dir := t.TempDir()
	writeFile(t, dir, ".creds.toml", "[map]\nDB = \"db/prod|url\"\n")
	t.Chdir(dir)
	h.ok(&fake{}, "trust", "--yes")
	var got map[string][]string
	r := h.ok(&fake{}, "trust", "list", "--json")
	if err := json.Unmarshal([]byte(r.out), &got); err != nil {
		t.Fatalf("%v: %q", err, r.out)
	}
	if len(got["trusted"]) != 1 {
		t.Fatalf("json %v", got)
	}
}
