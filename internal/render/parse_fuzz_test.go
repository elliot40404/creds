package render

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

func FuzzParseRender(f *testing.F) {
	seeds := []string{
		"",
		"postgres://bob:p%40ss@db.local:5433/app?sslmode=require",
		"postgresql://[::1]:5432/app",
		"postgres:///app?host=/tmp",
		"postgres://h1:1,h2:2/app",
		`host=db port=5432 user=bob password='a\'b' dbname=app`,
		`password=a\ b\\c dbname=''`,
		"redis://:secret@cache:6380/2",
		"rediss://u:p@cache",
		"mongodb+srv://u:p@cluster/db?retryWrites=true",
		"mongodb://a,b,c/db",
		"mongodb://u:p@host/db?retryWrites=true",
		"rediss://cache:6379/0?x=1",
		"host=db options=x://y",
		"postgres://日本:語@host/db",
		"postgres://u:$(id)`x`@h/d;rm -rf ?a=b&a=c",
		"postgres://",
		"mongodb://0 ",
		"postgres://:,0",
		"postgres://%2Ftmp/d",
		"postgres://h:0/d",
		"postgres://h:+1/d",
		"postgres://[::1/d",
		"redis://h/../../x",
		"mongodb://h/%zz",
		"postgres://h/d#frag",
		"host='unterminated",
		"=cmd",
		"\x00",
		"postgres://u:p\nq@h/d",
		strings.Repeat("a=b ", 1000),
	}
	for _, s := range seeds {
		for _, e := range []string{Postgres, Redis, Mongo} {
			f.Add(e, s)
		}
	}
	renderers := map[Shell]*Renderer{}
	for _, sh := range []Shell{Bash, Pwsh} {
		r, err := New(nil, sh)
		if err != nil {
			f.Fatal(err)
		}
		renderers[sh] = r
	}
	f.Fuzz(func(t *testing.T, engine, s string) {
		if want, ok := EngineFor(s); ok && want != engine {
			if _, err := Parse(engine, s); err == nil {
				t.Fatalf("%q is %s but parsed clean as %s", s, want, engine)
			}
		}
		c, err := Parse(engine, s)
		if err != nil {
			if !errors.Is(err, ErrInvalidConn) && !errors.Is(err, ErrUnknownEngine) {
				t.Fatalf("unexpected error type: %v", err)
			}
			return
		}
		for _, r := range renderers {
			renderAll(t, r, engine, c)
		}
		checkPwsh(t, renderers[Pwsh], engine, c)
		if c.Scheme != "" {
			roundtrip(t, renderers[Bash], engine, c)
		}
	})
}

func roundtrip(t *testing.T, r *Renderer, engine string, c Conn) {
	t.Helper()
	s, err := r.Render(engine, "url", c)
	if err != nil {
		t.Fatal(err)
	}
	got, err := Parse(engine, s)
	if err != nil {
		t.Fatalf("reparse %q: %v", s, err)
	}
	if !reflect.DeepEqual(got, c) {
		t.Fatalf("roundtrip %q: got %+v, want %+v", s, got, c)
	}
}

func renderAll(t *testing.T, r *Renderer, engine string, c Conn) {
	t.Helper()
	formats, err := r.Formats(engine)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range formats {
		if _, err := r.Render(engine, name, c); err != nil {
			t.Fatalf("render %s.%s %+v: %v", engine, name, c, err)
		}
	}
}
