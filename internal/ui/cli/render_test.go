package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGetAs(t *testing.T) {
	t.Parallel()
	h := seeded(t)
	h.writeConfig("[render]\nshell = \"bash\"\n")
	cases := map[string]string{
		"url":  dbConn,
		"psql": "PGPASSWORD=" + dbHidden + " PGSSLMODE=require psql -h db.example -p 5432 -U bob -d app",
		"env":  "PGHOST=db.example\nPGPORT=5432\nPGUSER=bob\nPGPASSWORD=" + dbHidden + "\nPGDATABASE=app\nPGSSLMODE=require",
	}
	for as, want := range cases {
		r := h.ok(&fake{}, "get", "db/prod", "--as", as)
		if r.out != want+"\n" || r.err != "" {
			t.Fatalf("%s: out %q err %q", as, r.out, r.err)
		}
	}
	r := h.fail(&fake{}, "get", "db/prod", "--as", "nope")
	if !strings.Contains(r.err, "unknown format") {
		t.Fatalf("err %q", r.err)
	}
	r = h.fail(&fake{}, "get", "web/mail", "--as", "url")
	if !strings.Contains(r.err, "not a database entry") {
		t.Fatalf("err %q", r.err)
	}
	noLeak(t, r, loginHidden)
	h.fail(&fake{}, "get", "db/prod", "--as", "url", "--field", "host")
	h.writeConfig("[render]\nshell = \"pwsh\"\n")
	r = h.ok(&fake{}, "get", "db/prod", "--as", "psql")
	if !strings.HasPrefix(r.out, "$env:PGPASSWORD=") {
		t.Fatalf("pwsh out %q", r.out)
	}
}

func TestGetFormats(t *testing.T) {
	t.Parallel()
	h := seeded(t)
	r := h.ok(&fake{}, "get", "db/prod", "--formats")
	if r.out != "dotenv\nenv\npg_dump\npg_restore\npsql\nurl\n" {
		t.Fatalf("out %q", r.out)
	}
	h.fail(&fake{}, "get", "db/prod", "--formats", "--as", "url")
}

func TestRenderConfigOverride(t *testing.T) {
	t.Parallel()
	h := seeded(t)
	cfg := "[render.formats]\n\"postgres.jdbc\" = \"jdbc:postgresql://{{.host}}:{{.port}}/{{.database}}\"\n"
	h.writeConfig(cfg)
	r := h.ok(&fake{}, "get", "db/prod", "--as", "jdbc")
	if r.out != "jdbc:postgresql://db.example:5432/app\n" {
		t.Fatalf("out %q", r.out)
	}
	if r = h.ok(&fake{}, "get", "db/prod", "--formats"); !strings.Contains(r.out, "jdbc\n") {
		t.Fatalf("formats %q", r.out)
	}
	h.writeConfig("[render.formats]\njdbc = \"x\"\n")
	r = h.fail(&fake{}, "get", "db/prod", "--as", "url")
	if !strings.Contains(r.err, "render.formats") {
		t.Fatalf("err %q", r.err)
	}
}

func TestFormatsEscapeNames(t *testing.T) {
	t.Parallel()
	h := seeded(t)
	h.writeConfig("[render.formats]\n\"postgres.x\\u001b]0;PWNED\\u0007\" = \"x\"\n")
	r := h.fail(&fake{}, "get", "db/prod", "--formats")
	if strings.ContainsAny(r.out+r.err, "\x1b\a") || !strings.Contains(r.err, "render.formats") {
		t.Fatalf("out %q err %q", r.out, r.err)
	}
}

func TestCopyAs(t *testing.T) {
	t.Parallel()
	h := seeded(t)
	r := h.ok(&fake{}, "copy", "db/prod", "--as", "url")
	if h.clip.value != dbConn || r.out != "" || r.err != "Copied db/prod as url\n" {
		t.Fatalf("out %q err %q", r.out, r.err)
	}
	noLeak(t, r, dbHidden)
	h.fail(&fake{}, "copy", "db/prod", "--as", "url", "--field", "host")
	h.fail(&fake{}, "copy", "web/mail", "--as", "url")
}

func (h *harness) writeConfig(body string) {
	h.t.Helper()
	if err := os.WriteFile(filepath.Join(h.home, "config.toml"), []byte(body), 0o600); err != nil {
		h.t.Fatal(err)
	}
}

func TestGetShellOverride(t *testing.T) {
	t.Parallel()
	h := seeded(t)
	h.writeConfig("[render]\nshell = \"bash\"\n")
	r := h.ok(&fake{}, "get", "db/prod", "--as", "psql", "--shell", "pwsh")
	if !strings.HasPrefix(r.out, "$env:PGPASSWORD=") {
		t.Fatalf("out %q", r.out)
	}
	r = h.ok(&fake{}, "get", "db/prod", "--as", "psql")
	if strings.Contains(r.out, "$env:") {
		t.Fatalf("default out %q", r.out)
	}
	h.fail(&fake{}, "get", "db/prod", "--as", "psql", "--shell", "fish")
	h.fail(&fake{}, "get", "db/prod", "--shell", "bash")
}

func TestCopyShellOverride(t *testing.T) {
	t.Parallel()
	h := seeded(t)
	h.writeConfig("[render]\nshell = \"bash\"\n")
	r := h.ok(&fake{}, "copy", "db/prod", "--as", "psql", "--shell", "pwsh")
	if !strings.HasPrefix(h.clip.value, "$env:PGPASSWORD=") {
		t.Fatalf("clip %q", h.clip.value)
	}
	if r.err != "Copied db/prod as psql in pwsh\n" {
		t.Fatalf("err %q", r.err)
	}
	noLeak(t, r, dbHidden)
	h.fail(&fake{}, "copy", "db/prod", "--shell", "pwsh")
}

func TestListSort(t *testing.T) {
	t.Parallel()
	h := seeded(t)
	h.ok(&fake{passwords: []string{loginHidden}, inputs: []string{"zoe", "", ""}},
		"add", "--type", "login", "aaa/newest")
	byPath := paths(t, h.ok(&fake{}, "list").out)
	if byPath[0] != "aaa/newest" || len(byPath) != 3 {
		t.Fatalf("path order %q", byPath)
	}
	byCreated := paths(t, h.ok(&fake{}, "list", "--sort", "created").out)
	if byCreated[0] != "aaa/newest" {
		t.Fatalf("created order %q", byCreated)
	}
	byUpdated := paths(t, h.ok(&fake{}, "search", "a", "--sort", "updated").out)
	if len(byUpdated) == 0 {
		t.Fatalf("updated order %q", byUpdated)
	}
	r := h.fail(&fake{}, "list", "--sort", "nope")
	if !strings.Contains(r.err, "not a sort order") || !strings.Contains(r.err, "path|updated|created") {
		t.Fatalf("err %q", r.err)
	}
}

func paths(t *testing.T, out string) []string {
	t.Helper()
	var got []string
	for i, line := range strings.Split(strings.TrimSpace(out), "\n") {
		if i == 0 {
			continue
		}
		got = append(got, strings.Fields(line)[0])
	}
	return got
}
