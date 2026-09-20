package cli

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

const (
	envHidden = "h1dden-env-value"
	envSrc    = "API_KEY=" + envHidden + "\nMODE=dev\nMODE=prod\nNOTE=\"a b\"\n"
)

func writeFile(t *testing.T, dir, name, body string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func imported(t *testing.T) *harness {
	t.Helper()
	h := seeded(t)
	file := writeFile(t, t.TempDir(), ".env", envSrc)
	r := h.ok(&fake{}, "import-env", file, "proj/env")
	if r.out != "" || !strings.Contains(r.err, "warning: line 3: duplicate key MODE") || !strings.HasSuffix(r.err, "imported proj/env\n") {
		t.Fatalf("out %q err %q", r.out, r.err)
	}
	noLeak(t, r, envHidden)
	return h
}

func TestImportEnv(t *testing.T) {
	t.Parallel()
	h := imported(t)
	r := h.ok(&fake{}, "get", "proj/env")
	want := "path     proj/env\ntype     env\nAPI_KEY  " + mask + "\nMODE     " + mask + "\nNOTE     " + mask + "\n"
	if withoutStamps(r.out) != want {
		t.Fatalf("out %q", r.out)
	}
	noLeak(t, r, envHidden)
	h.fail(&fake{}, "import-env", filepath.Join(t.TempDir(), "missing"), "proj/x")
	bad := writeFile(t, t.TempDir(), ".env", "BAD KEY=1")
	h.fail(&fake{}, "import-env", bad, "proj/x")
	h.fail(&fake{}, "get", "proj/x")
}

func TestEnvStdout(t *testing.T) {
	t.Parallel()
	h := imported(t)
	r := h.ok(&fake{}, "env", "proj/env")
	if r.out != "API_KEY="+envHidden+"\nMODE=prod\nNOTE='a b'\n" || r.err != "" {
		t.Fatalf("out %q err %q", r.out, r.err)
	}
	r = h.fail(&fake{}, "env", "db/prod")
	if !strings.Contains(r.err, "not an env entry") {
		t.Fatalf("err %q", r.err)
	}
	noLeak(t, r, dbHidden)
}

func TestEnvFileRoundtrip(t *testing.T) {
	t.Parallel()
	h := imported(t)
	dir := t.TempDir()
	out := filepath.Join(dir, "out.env")
	r := h.ok(&fake{confirms: []bool{true}}, "env", "proj/env", "-o", out)
	if r.out != "" || r.err != "wrote "+out+"\n" {
		t.Fatalf("out %q err %q", r.out, r.err)
	}
	checkPrivate(t, out)
	h.ok(&fake{}, "import-env", out, "proj/copy")
	a := h.ok(&fake{}, "env", "proj/env").out
	if b := h.ok(&fake{}, "env", "proj/copy").out; a != b {
		t.Fatalf("roundtrip %q != %q", a, b)
	}

	skipped := filepath.Join(dir, "skip.env")
	h.fail(&fake{confirms: []bool{false}}, "env", "proj/env", "-o", skipped)
	if _, err := os.Stat(skipped); !os.IsNotExist(err) {
		t.Fatalf("declined file written: %v", err)
	}
	h.ok(&fake{}, "env", "proj/env", "--output", skipped, "--yes")
	checkPrivate(t, skipped)
}

func checkPrivate(t *testing.T, path string) {
	t.Helper()
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && fi.Mode().Perm() != 0o600 {
		t.Fatalf("mode %v", fi.Mode())
	}
}

func TestEnvProject(t *testing.T) {
	h := imported(t)
	dir := t.TempDir()
	writeFile(t, dir, ".creds.toml", "env = \"proj/env\"\n[map]\nMODE = \"db/prod:database\"\nDB = \"db/prod|url\"\n")
	t.Chdir(dir)
	r := h.ok(&fake{confirms: []bool{true}}, "env")
	want := "API_KEY=" + envHidden + "\nMODE=app\nNOTE='a b'\nDB='" + dbConn + "'\n"
	if r.out != want {
		t.Fatalf("out %q", r.out)
	}
	r = h.ok(&fake{}, "env", "proj/env")
	if strings.Contains(r.out, "DB=") || !strings.Contains(r.out, "MODE=prod") {
		t.Fatalf("arg ignored: %q", r.out)
	}
	writeFile(t, dir, ".creds.toml", "[map]\nX = \"db/prod:nope\"\n")
	r = h.fail(&fake{confirms: []bool{true}}, "env")
	if !strings.Contains(r.err, "map.X") || !strings.Contains(r.err, "no such field") {
		t.Fatalf("err %q", r.err)
	}
	t.Chdir(t.TempDir())
	r = h.fail(&fake{confirms: []bool{true}}, "env")
	if !strings.Contains(r.err, "no path given") {
		t.Fatalf("err %q", r.err)
	}
}
