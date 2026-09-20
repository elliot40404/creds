package cli

import (
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
)

const runHelperEnv = "CREDS_CLI_RUN_HELPER"

func TestRunHelperProcess(t *testing.T) {
	if os.Getenv(runHelperEnv) != "1" {
		t.Skip("helper only")
	}
	fmt.Printf("%s|%s", os.Getenv("API_KEY"), os.Getenv("DB"))
	code, _ := strconv.Atoi(os.Getenv("RUN_HELPER_EXIT"))
	os.Exit(code)
}

func runHelper(t *testing.T, exit string) []string {
	t.Helper()
	t.Setenv(runHelperEnv, "1")
	t.Setenv("RUN_HELPER_EXIT", exit)
	return []string{"--", os.Args[0], "-test.run=^TestRunHelperProcess$"}
}

func TestRunInjectsEnv(t *testing.T) {
	h := imported(t)
	r := h.ok(&fake{}, append([]string{"run", "proj/env"}, runHelper(t, "0")...)...)
	if r.out != envHidden+"|" || r.err != "" {
		t.Fatalf("out %q err %q", r.out, r.err)
	}
	if os.Getenv("API_KEY") != "" {
		t.Fatal("parent env changed")
	}
	dir := t.TempDir()
	writeFile(t, dir, ".creds.toml", "[map]\nDB = \"db/prod|url\"\n")
	t.Chdir(dir)
	r = h.ok(&fake{confirms: []bool{true}}, append([]string{"run"}, runHelper(t, "0")...)...)
	if r.out != "|"+dbConn {
		t.Fatalf("out %q", r.out)
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 1 {
		t.Fatalf("run wrote files: %v %v", entries, err)
	}
}

func TestRunExitCode(t *testing.T) {
	h := imported(t)
	r := h.run(&fake{}, append([]string{"run", "proj/env"}, runHelper(t, "3")...)...)
	if r.code != 3 || r.err != "" {
		t.Fatalf("code %d err %q", r.code, r.err)
	}
}

func TestRunLockEndsSession(t *testing.T) {
	h := imported(t)
	r := h.ok(&fake{}, append([]string{"run", "--lock", "proj/env"}, runHelper(t, "0")...)...)
	if r.out != envHidden+"|" {
		t.Fatalf("out %q", r.out)
	}
	h.fail(&fake{}, "unlock")
}

func TestRunRejects(t *testing.T) {
	h := imported(t)
	for _, args := range [][]string{
		{"run", "proj/env"},
		{"run", "proj/env", "--"},
		{"run", "a", "b", "--", "echo"},
	} {
		r := h.fail(&fake{}, args...)
		if r.code != 2 || !strings.Contains(r.err, "missing --") {
			t.Fatalf("%v: %q", args, r.err)
		}
	}
	r := h.fail(&fake{}, append([]string{"run", "missing"}, runHelper(t, "0")...)...)
	if !strings.Contains(r.err, "not found") || r.out != "" {
		t.Fatalf("missing: out %q err %q", r.out, r.err)
	}
	r = h.fail(&fake{}, append([]string{"run", "db/prod"}, runHelper(t, "0")...)...)
	if r.out != "" {
		t.Fatalf("non env ran: %q", r.out)
	}
	r = h.fail(&fake{}, "run", "proj/env", "--", "creds-no-such-binary-xyz")
	want := "error: creds-no-such-binary-xyz: program not found\nfix: check the program name"
	if r.code != 4 || !strings.HasPrefix(r.err, want) {
		t.Fatalf("no program: %d %q", r.code, r.err)
	}
}

func TestRunnerOnlyFromRunCmd(t *testing.T) {
	t.Parallel()
	const runner = "github.com/elliot40404/creds/internal/runner"
	for _, dir := range []string{".", "../../app", "../../vault", "../../render", "../../envfile", "../../clipboard"} {
		files, err := filepath.Glob(filepath.Join(dir, "*.go"))
		if err != nil {
			t.Fatal(err)
		}
		for _, name := range files {
			if strings.HasSuffix(name, "_test.go") || name == "runcmd.go" {
				continue
			}
			if slices.Contains(imports(t, name), runner) {
				t.Fatalf("%s imports runner", name)
			}
		}
	}
	for _, imp := range imports(t, "../../runner/runner.go") {
		if strings.HasPrefix(imp, "github.com/elliot40404/creds/") {
			t.Fatalf("runner imports %s", imp)
		}
	}
}

func imports(t *testing.T, name string) []string {
	t.Helper()
	f, err := parser.ParseFile(token.NewFileSet(), name, nil, parser.ImportsOnly)
	if err != nil {
		t.Fatal(err)
	}
	out := make([]string, len(f.Imports))
	for i, imp := range f.Imports {
		out[i] = strings.Trim(imp.Path.Value, `"`)
	}
	return out
}
