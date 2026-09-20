package envfile

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

var weirdSeeds = []string{
	"",
	"A=1",
	"A=\"x\\ny\"\nB='q'\n",
	"export A=b # c\n",
	"A=héllo wörld 日本",
	"A=\"unterminated",
	"A='a\r\nb'",
	"A=x\x00y",
	"A=../../etc/passwd",
	"A==cmd|' /C calc'!A0",
	"A=\"\\$HOME `id`\"",
	"=\n",
	"export =1",
	"1A=2",
	"A = \"a\" junk",
	"A=\\",
	"A=\"\\",
	strings.Repeat("K=v\n", 500),
	"A=" + strings.Repeat("x", 4096),
}

func FuzzParse(f *testing.F) {
	for _, s := range weirdSeeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, src string) {
		vars, _, err := Parse(strings.NewReader(src))
		if err != nil {
			if vars != nil {
				t.Fatalf("vars on error: %v", err)
			}
			return
		}
		assertRoundtrip(t, vars)
	})
}

func FuzzWrite(f *testing.F) {
	for _, s := range weirdSeeds {
		f.Add("KEY", s)
	}
	f.Add("export", "x")
	f.Add("_", "#x")
	f.Fuzz(func(t *testing.T, key, val string) {
		vars := []Var{{Key: key, Value: val}}
		for _, write := range []func(io.Writer, []Var) error{Write, WriteDotenv} {
			var buf bytes.Buffer
			if err := write(&buf, vars); err != nil {
				continue
			}
			got, _, err := Parse(&buf)
			if err != nil {
				t.Fatalf("parse written %q: %v", buf.String(), err)
			}
			if !slices.Equal(got, vars) {
				t.Fatalf("roundtrip: got %q want %q", got, vars)
			}
		}
	})
}

func assertRoundtrip(t *testing.T, vars []Var) {
	t.Helper()
	var buf bytes.Buffer
	if err := Write(&buf, vars); err != nil {
		t.Fatalf("write parsed vars: %v", err)
	}
	out := buf.String()
	got, _, err := Parse(&buf)
	if err != nil {
		t.Fatalf("reparse %q: %v", out, err)
	}
	if !slices.Equal(got, vars) {
		t.Fatalf("roundtrip: got %q want %q", got, vars)
	}
}

func FuzzParseRef(f *testing.F) {
	for _, s := range []string{"", "a:b", "a|b", "a/b:c|d", " a:b", "a:", ":b", "../x:y", "日本:語", "a:b\n", "a\x00:b"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		ref, err := ParseRef(s)
		if err != nil {
			return
		}
		sep, sel := ":", ref.Field
		if ref.Format != "" {
			sep, sel = "|", ref.Format
		}
		again, err := ParseRef(ref.Path + sep + sel)
		if err != nil || again != ref {
			t.Fatalf("ref roundtrip %q: %+v %+v %v", s, ref, again, err)
		}
	})
}

func FuzzLoadProject(f *testing.F) {
	for _, s := range []string{
		"",
		"env = \".env\"",
		"env = \"\"",
		"[map]\nDB = \"db/main:password\"",
		"[map]\n\"1X\" = \"a:b\"",
		"[map]\nX = \"a|url\"\nY = 3",
		"env = \"../../etc\"\nother = 1",
		"env = \"a\x00b\"",
		"env = '''\nx'''",
		"[[map]]",
	} {
		f.Add(s)
	}
	dir := f.TempDir()
	path := filepath.Join(dir, ProjectFile)
	f.Fuzz(func(t *testing.T, src string) {
		if err := os.WriteFile(path, []byte(src), 0o600); err != nil {
			t.Fatal(err)
		}
		p, err := LoadProject(dir)
		if err != nil {
			return
		}
		if p.Env == "" && len(p.Map) == 0 {
			t.Fatalf("empty project accepted: %q", src)
		}
		for k := range p.Map {
			if !validKey(k) {
				t.Fatalf("bad key accepted: %q", k)
			}
		}
	})
}
