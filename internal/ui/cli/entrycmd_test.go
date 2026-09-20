package cli

import (
	"strings"
	"testing"
)

const (
	loginHidden = "h1dden-login-value"
	dbHidden    = "h1dden-db-value"
	extraHidden = "h1dden-extra-value"
	newHidden   = "n3w-db-value"
	dbConn      = "postgres://bob:" + dbHidden + "@db.example:5432/app?sslmode=require"
)

func seeded(t *testing.T) *harness {
	t.Helper()
	h := newHarness(t)
	h.init()
	h.ok(&fake{
		passwords: []string{loginHidden},
		inputs:    []string{"alice", "https://mail.example", ""},
	}, "add", "--type", "login", "web/mail")
	h.ok(&fake{
		passwords: []string{dbConn, extraHidden},
		inputs:    []string{"token_ref", "region", "eu-west", ""},
		confirms:  []bool{true, false},
	}, "add", "--type", "database", "db/prod")
	return h
}

func noLeak(t *testing.T, r result, hidden ...string) {
	t.Helper()
	for _, s := range hidden {
		if strings.Contains(r.out, s) || strings.Contains(r.err, s) {
			t.Fatalf("output leaks %q:\n%s\n%s", s, r.out, r.err)
		}
	}
}

func TestAddSelectsType(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.init()
	r := h.ok(&fake{selects: []int{5}, inputs: []string{"make deploy", ""}}, "add", "ops/deploy")
	if r.out != "" || !strings.Contains(r.err, "added ops/deploy") {
		t.Fatalf("out %q err %q", r.out, r.err)
	}
	r = h.ok(&fake{}, "get", "--field", "command", "ops/deploy")
	if r.out != "make deploy\n" {
		t.Fatalf("out = %q", r.out)
	}
}

func TestAddRejects(t *testing.T) {
	t.Parallel()
	h := seeded(t)
	h.fail(&fake{}, "add", "--type", "bogus", "x")
	h.fail(&fake{inputs: []string{"u", "", ""}, passwords: []string{"v"}}, "add", "--type", "login", "web/mail")
	h.fail(&fake{selects: []int{0}, passwords: []string{"mysql://x"}}, "add", "--type", "database", "db/bad")
	h.fail(&fake{inputs: []string{"c", "command"}}, "add", "--type", "command", "dup/field")
}

func TestAddAsksPasswordFirstWhenLocked(t *testing.T) {
	t.Parallel()
	h := seeded(t)
	h.ok(&fake{}, "lock")
	f := &fake{passwords: []string{mainWord}, inputs: []string{"note body", ""}}
	h.ok(f, "add", "--type", "note", "misc/n")
	if f.prompts[0] != "Master password" {
		t.Fatalf("prompts = %v", f.prompts)
	}
}

func TestGetMasksSecrets(t *testing.T) {
	t.Parallel()
	h := seeded(t)
	r := h.ok(&fake{}, "get", "db/prod")
	noLeak(t, r, dbHidden, extraHidden)
	for _, want := range []string{"db.example", "bob", "5432", "sslmode", "require", "region", "eu-west", mask} {
		if !strings.Contains(r.out, want) {
			t.Fatalf("missing %q in %q", want, r.out)
		}
	}
	if r.err != "" {
		t.Fatalf("stderr = %q", r.err)
	}
	r = h.ok(&fake{}, "get", "web/mail")
	noLeak(t, r, loginHidden)
}

func TestGetShow(t *testing.T) {
	t.Parallel()
	h := seeded(t)
	r := h.ok(&fake{}, "get", "--show", "db/prod")
	for _, want := range []string{dbHidden, extraHidden} {
		if !strings.Contains(r.out, want) {
			t.Fatalf("missing %q", want)
		}
	}
}

func TestGetField(t *testing.T) {
	t.Parallel()
	h := seeded(t)
	cases := map[string]string{
		"password":  dbHidden,
		"token_ref": extraHidden,
		"username":  "bob",
		"sslmode":   "require",
		"engine":    "postgres",
	}
	for field, want := range cases {
		r := h.ok(&fake{}, "get", "--field", field, "db/prod")
		if r.out != want+"\n" {
			t.Fatalf("%s = %q", field, r.out)
		}
	}
	h.fail(&fake{}, "get", "--field", "nope", "db/prod")
	h.fail(&fake{}, "get", "db/missing")
}

func TestListAndSearch(t *testing.T) {
	t.Parallel()
	h := seeded(t)
	r := h.ok(&fake{}, "list")
	noLeak(t, r, loginHidden, dbHidden, extraHidden)
	lines := strings.Split(strings.TrimSpace(r.out), "\n")
	if len(lines) != 3 || !strings.HasPrefix(lines[1], "db/prod") || !strings.Contains(lines[1], "db.example") {
		t.Fatalf("list = %q", r.out)
	}
	if !strings.Contains(lines[2], "alice") {
		t.Fatalf("list = %q", r.out)
	}
	r = h.ok(&fake{}, "search", "mail")
	noLeak(t, r, loginHidden)
	if !strings.Contains(r.out, "web/mail") || strings.Contains(r.out, "db/prod") {
		t.Fatalf("search = %q", r.out)
	}
	h.fail(&fake{}, "search")
}

func TestEditKeepsBlankSecret(t *testing.T) {
	t.Parallel()
	h := seeded(t)
	h.ok(&fake{
		inputs:    []string{"", "", "https://new.example", ""},
		passwords: []string{""},
	}, "edit", "web/mail")
	r := h.ok(&fake{}, "get", "--field", "password", "web/mail")
	if r.out != loginHidden+"\n" {
		t.Fatalf("password = %q", r.out)
	}
	r = h.ok(&fake{}, "get", "--field", "url", "web/mail")
	if r.out != "https://new.example\n" {
		t.Fatalf("url = %q", r.out)
	}
}

func TestEditRenameAndChangeSecret(t *testing.T) {
	t.Parallel()
	h := seeded(t)
	h.ok(&fake{
		inputs:    []string{"db/main", "", "", "", "", "", "", "eu-north", "api_hint", "abc", ""},
		passwords: []string{newHidden, ""},
		confirms:  []bool{false},
	}, "edit", "db/prod")
	h.fail(&fake{}, "get", "db/prod")
	cases := map[string]string{"password": newHidden, "token_ref": extraHidden, "region": "eu-north", "api_hint": "abc"}
	for field, want := range cases {
		r := h.ok(&fake{}, "get", "--field", field, "db/main")
		if r.out != want+"\n" {
			t.Fatalf("%s = %q", field, r.out)
		}
	}
}

func TestRm(t *testing.T) {
	t.Parallel()
	h := seeded(t)
	r := h.fail(&fake{confirms: []bool{false}}, "rm", "web/mail")
	if !strings.Contains(r.err, "aborted") {
		t.Fatalf("err = %q", r.err)
	}
	h.ok(&fake{}, "get", "web/mail")
	h.ok(&fake{confirms: []bool{true}}, "rm", "web/mail")
	h.fail(&fake{}, "get", "web/mail")
	h.ok(&fake{}, "rm", "--yes", "db/prod")
	h.fail(&fake{}, "rm", "--yes", "db/prod")
	r = h.ok(&fake{}, "list")
	if strings.Count(r.out, "\n") != 1 {
		t.Fatalf("list = %q", r.out)
	}
}

func TestAddDetectsTheEngineFromTheConnString(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.init()
	h.withStdin(t, "mongodb://u:p@mongo.host/app?retryWrites=true\n",
		"add", "db/auto", "--type", "database", "--conn", "-")
	for _, tc := range [][2]string{{"engine", "mongo"}, {"host", "mongo.host"}, {"database", "app"}} {
		out := h.ok(&fake{}, "get", "db/auto", "--field", tc[0]).out
		if strings.TrimSpace(out) != tc[1] {
			t.Fatalf("%s = %q, want %q", tc[0], out, tc[1])
		}
	}
}

func TestAddEngineFlagBeatsDetection(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.init()
	h.failStdin(t, "mongodb://u:p@h/app\n", "add", "db/clash", "--type", "database", "--engine", "redis", "--conn", "-")
}

func TestAddNeedsAnEngineWhenUndetectable(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.init()
	r := h.failStdin(t, "sslmode=require\n", "add", "db/nope", "--type", "database", "--conn", "-")
	if !strings.Contains(r.err, "cannot tell the database engine") {
		t.Fatalf("err %q", r.err)
	}
	if !strings.Contains(r.err, "postgres|redis|mongo") {
		t.Fatalf("hint %q", r.err)
	}
}

func withoutStamps(out string) string {
	var keep []string
	for line := range strings.SplitSeq(out, "\n") {
		switch strings.Fields(line + " x")[0] {
		case "created", "updated", "machine", "versions":
			continue
		}
		keep = append(keep, line)
	}
	return strings.Join(keep, "\n")
}

func TestGetShowsWhenWhereAndVersions(t *testing.T) {
	t.Parallel()
	h := seeded(t)
	out := h.ok(&fake{}, "get", "web/mail").out
	for _, want := range []string{"created ", "updated ", "machine "} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in %q", want, out)
		}
	}
	if strings.Contains(out, "versions") {
		t.Fatalf("a fresh entry claims versions: %q", out)
	}
	h.ok(&fake{inputs: []string{"", "", "https://new.example", ""}, passwords: []string{""}}, "edit", "web/mail")
	out = h.ok(&fake{}, "get", "web/mail").out
	if !strings.Contains(out, "versions  1 kept") {
		t.Fatalf("no version count after an edit: %q", out)
	}
}
