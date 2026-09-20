package cli

import (
	"encoding/json/v2"
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/elliot40404/creds/internal/fail"
	"github.com/elliot40404/creds/internal/safetext"
)

func decode(t *testing.T, s string) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		t.Fatalf("not json %q: %v", s, err)
	}
	return m
}

func keys(m map[string]any) string {
	return strings.Join(slices.Sorted(maps.Keys(m)), ",")
}

func TestJSONSchemas(t *testing.T) {
	t.Parallel()
	h := imported(t)
	cases := []struct {
		args []string
		keys string
	}{
		{[]string{"get", "web/mail"}, "created,fields,path,tags,type,updated,url,username"},
		{[]string{"get", "web/mail", "--field", "username"}, "field,path,value"},
		{[]string{"get", "db/prod", "--as", "url"}, "format,path,value"},
		{[]string{"get", "db/prod", "--formats"}, "formats,path"},
		{[]string{"list"}, "entries"},
		{[]string{"search", "mail"}, "entries"},
		{[]string{"status"}, "ahead,behind,conflicts,generation,remote,trusted"},
		{[]string{"env", "proj/env"}, "API_KEY,MODE,NOTE"},
	}
	for _, c := range cases {
		r := h.ok(&fake{}, append(c.args, "--json")...)
		if got := keys(decode(t, r.out)); got != c.keys {
			t.Errorf("%v: keys %s", c.args, got)
		}
		if r.err != "" {
			t.Errorf("%v: stderr %q", c.args, r.err)
		}
	}
	list := decode(t, h.ok(&fake{}, "list", "--json").out)["entries"].([]any)
	if got := keys(list[0].(map[string]any)); got != "created,engine,host,path,tags,type,updated,username" {
		t.Errorf("summary keys %s", got)
	}
}

func TestJSONHidesSecrets(t *testing.T) {
	t.Parallel()
	h := seeded(t)
	r := h.ok(&fake{}, "get", "web/mail", "--json")
	noLeak(t, r, loginHidden)
	fields := decode(t, r.out)["fields"].([]any)
	pw := fields[0].(map[string]any)
	if keys(pw) != "name,secret" || pw["secret"] != true {
		t.Fatalf("masked field %v", pw)
	}
	r = h.ok(&fake{}, "get", "web/mail", "--json", "--show")
	if !strings.Contains(r.out, loginHidden) {
		t.Fatalf("show %q", r.out)
	}
}

func TestJSONErrorsAndNotes(t *testing.T) {
	t.Parallel()
	h := seeded(t)
	for _, args := range [][]string{{"get", "nope", "--json"}, {"--json", "bogus"}, {"get", "--bad", "--json"}} {
		r := h.run(&fake{}, args...)
		m := decode(t, r.err)
		if r.out != "" || keys(m) != "code,error,hint" || int(m["code"].(float64)) != r.code || r.code == 0 {
			t.Errorf("%v: %d out %q err %q", args, r.code, r.out, r.err)
		}
	}
	if r := h.run(&fake{}, "get", "nope", "--json"); r.code != int(fail.NotFound) {
		t.Fatalf("code %d", r.code)
	}
	r := h.stdin("", "add", "x/n", "--type", "note", "--notes", "hi", "--json")
	if r.code != 0 || decode(t, r.err)["note"] != "added x/n" {
		t.Fatalf("note %q", r.err)
	}
}

func TestJSONTrustAndSync(t *testing.T) {
	t.Parallel()
	h := imported(t)
	dir := t.TempDir()
	writeFile(t, dir, ".creds.toml", "[map]\nDB = \"db/prod|url\"\n")
	r := h.ok(&fake{}, "trust", "--yes", "--json", dir)
	if m := decode(t, r.out); keys(m) != "file,refs" || len(m["refs"].([]any)) != 1 {
		t.Fatalf("trust %q", r.out)
	}
	a, b := joined(t)
	b.editDeploy("theirs")
	b.ok(&fake{}, "sync")
	a.editDeploy("mine")
	r = a.run(&fake{}, "sync", "--json")
	if m := decode(t, r.out); r.code != int(fail.Conflict) || m["result"] != "conflict" || len(m["conflicts"].([]any)) != 1 {
		t.Fatalf("conflict %d %q %q", r.code, r.out, r.err)
	}
	if decode(t, r.err)["code"] != float64(fail.Conflict) {
		t.Fatalf("stderr %q", r.err)
	}
	r = a.ok(&fake{}, "resolve", "ops/deploy", "--mine", "--yes", "--json")
	if m := decode(t, r.out); keys(m) != "conflicts,result" {
		t.Fatalf("resolve %q", r.out)
	}
}

func TestJSONEscapesInvisibleRunes(t *testing.T) {
	t.Parallel()
	in := map[string]string{"v": "a" + string(rune(0x202e)) + "b" + string(rune(0x200b)) + "c" + string(rune(0x2028)) + "d" + string(rune(0x7f)) + "e" + string(rune(0xe0041)) + "f" + string(rune(0xe9))}
	var b strings.Builder
	if err := writeJSONLine(&b, in); err != nil {
		t.Fatal(err)
	}
	got := b.String()
	for _, r := range got {
		if safetext.IsControl(r) && r != '\n' {
			t.Fatalf("raw %U in %q", r, got)
		}
	}
	if !strings.Contains(got, esc("202e")) || !strings.Contains(got, esc("db40")+esc("dc41")) {
		t.Fatalf("got %q", got)
	}
	var back map[string]string
	if err := json.Unmarshal([]byte(got), &back); err != nil || back["v"] != in["v"] {
		t.Fatalf("round trip %q %v", back["v"], err)
	}
}

func esc(hex string) string {
	return `\` + "u" + hex
}
