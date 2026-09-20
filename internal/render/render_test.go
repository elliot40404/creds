package render

import (
	"errors"
	"reflect"
	"slices"
	"strings"
	"testing"
)

const trickyEsc = "p%40ss%3Aw%2Frd%27%22"

func newRenderer(t *testing.T, overrides map[string]string) *Renderer {
	t.Helper()
	r, err := New(overrides, Bash)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestRenderOutputs(t *testing.T) {
	r := newRenderer(t, nil)
	pg := Conn{Host: "db.local", Port: "5432", Username: "bob", Password: tricky, Database: "app", Params: map[string]string{"sslmode": "require"}}
	rd := Conn{Scheme: "rediss", Host: "cache", Port: "6380", Username: "u", Password: tricky, Database: "2"}
	mg := Conn{Scheme: "mongodb+srv", Host: "c.example.com", Username: "u", Password: tricky, Database: "app"}
	pgURL := "postgresql://bob:" + trickyEsc + "@db.local:5432/app?sslmode=require"
	mgURL := "mongodb+srv://u:" + trickyEsc + "@c.example.com/app"
	pgPrefix := `PGPASSWORD='p@ss:w/rd'\''"' PGSSLMODE=require `
	pgArgs := " -h db.local -p 5432 -U bob -d app"
	cases := []struct {
		engine, format string
		c              Conn
		want           string
	}{
		{Postgres, "url", pg, pgURL},
		{Postgres, "env", pg, "PGHOST=db.local\nPGPORT=5432\nPGUSER=bob\nPGPASSWORD='p@ss:w/rd'\\''\"'\nPGDATABASE=app\nPGSSLMODE=require"},
		{Postgres, "psql", pg, pgPrefix + "psql" + pgArgs},
		{Postgres, "pg_dump", pg, pgPrefix + "pg_dump" + pgArgs},
		{Postgres, "pg_restore", pg, pgPrefix + "pg_restore" + pgArgs},
		{Postgres, "psql", Conn{Host: "h"}, "psql -h h"},
		{Postgres, "env", Conn{}, ""},
		{Redis, "url", rd, "rediss://u:" + trickyEsc + "@cache:6380/2"},
		{Redis, "redis-cli", rd, `REDISCLI_AUTH='p@ss:w/rd'\''"' redis-cli -h cache -p 6380 --user u -n 2 --tls`},
		{Redis, "redis-cli", Conn{Host: "cache"}, "redis-cli -h cache"},
		{Redis, "env", Conn{Host: "cache"}, "REDIS_URL=redis://cache"},
		{Mongo, "url", mg, mgURL},
		{Mongo, "mongosh", mg, "mongosh " + mgURL},
		{Mongo, "mongodump", mg, "mongodump --uri=" + mgURL},
		{Mongo, "mongorestore", mg, "mongorestore --uri=" + mgURL},
		{Mongo, "env", mg, "MONGODB_URI=" + mgURL},
		{Mongo, "mongosh", Conn{Host: "h", Params: map[string]string{"a": "1", "b": "2"}}, "mongosh 'mongodb://h/?a=1&b=2'"},
	}
	for _, tc := range cases {
		t.Run(tc.engine+"."+tc.format, func(t *testing.T) {
			got, err := r.Render(tc.engine, tc.format, tc.c)
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Fatalf("got  %s\nwant %s", got, tc.want)
			}
		})
	}
}

func TestRenderRoundtrip(t *testing.T) {
	r := newRenderer(t, nil)
	cases := map[string]string{
		Postgres: "postgresql://b%20ob:" + trickyEsc + "@[::1]:5433/my%20db?application_name=x+y&sslmode=verify-full",
		Redis:    "rediss://:" + trickyEsc + "@cache:6380/3",
		Mongo:    "mongodb://u:" + trickyEsc + "@h1:27017,h2:27018/app?authSource=admin&replicaSet=rs0",
	}
	for engine, in := range cases {
		t.Run(engine, func(t *testing.T) {
			first, err := Parse(engine, in)
			if err != nil {
				t.Fatal(err)
			}
			if first.Password != tricky {
				t.Fatalf("password %q", first.Password)
			}
			out, err := r.Render(engine, "url", first)
			if err != nil {
				t.Fatal(err)
			}
			second, err := Parse(engine, out)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(first, second) {
				t.Fatalf("first %+v\nsecond %+v", first, second)
			}
		})
	}
}

func TestRenderDSNToURL(t *testing.T) {
	r := newRenderer(t, nil)
	c, err := Parse(Postgres, `host=h user=bob password='p@ss:w/rd\'"' dbname=app`)
	if err != nil {
		t.Fatal(err)
	}
	got, err := r.Render(Postgres, "url", c)
	if err != nil {
		t.Fatal(err)
	}
	if want := "postgresql://bob:" + trickyEsc + "@h/app"; got != want {
		t.Fatalf("got %s, want %s", got, want)
	}
}

func TestRenderUnknown(t *testing.T) {
	r := newRenderer(t, nil)
	if _, err := r.Render(Postgres, "nope", Conn{}); !errors.Is(err, ErrUnknownFormat) {
		t.Fatalf("err %v", err)
	}
	if _, err := r.Render("mysql", "url", Conn{}); !errors.Is(err, ErrUnknownEngine) {
		t.Fatalf("err %v", err)
	}
	if _, err := r.Formats("mysql"); !errors.Is(err, ErrUnknownEngine) {
		t.Fatalf("err %v", err)
	}
}

func TestFormats(t *testing.T) {
	r := newRenderer(t, map[string]string{"redis.go": "{{.url}}"})
	want := map[string][]string{
		Postgres: {"dotenv", "env", "pg_dump", "pg_restore", "psql", "url"},
		Redis:    {"dotenv", "env", "go", "redis-cli", "url"},
		Mongo:    {"dotenv", "env", "mongodump", "mongorestore", "mongosh", "url"},
	}
	for engine, w := range want {
		got, err := r.Formats(engine)
		if err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(got, w) {
			t.Fatalf("%s: got %v, want %v", engine, got, w)
		}
	}
}

func TestOverrides(t *testing.T) {
	r := newRenderer(t, map[string]string{
		"postgres.url":   "pg://{{.username}}@{{.host}}",
		"postgres.jdbc":  "jdbc:postgresql://{{.host}}:{{.port}}/{{pathesc .database}}?user={{queryesc .username}}",
		"mongo.password": "{{sh .password}}",
	})
	c := Conn{Host: "h", Port: "1", Username: "a&b", Password: "x y", Database: "d b"}
	cases := map[[2]string]string{
		{Postgres, "url"}:   "pg://a&b@h",
		{Postgres, "jdbc"}:  "jdbc:postgresql://h:1/d%20b?user=a%26b",
		{Mongo, "password"}: "'x y'",
		{Postgres, "psql"}:  "PGPASSWORD='x y' psql -h h -p 1 -U 'a&b' -d 'd b'",
	}
	for k, want := range cases {
		got, err := r.Render(k[0], k[1], c)
		if err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Fatalf("%v: got %s, want %s", k, got, want)
		}
	}
	base := newRenderer(t, nil)
	if got, _ := base.Render(Postgres, "url", c); got == "pg://a&b@h" {
		t.Fatal("override leaked into new renderer")
	}
}

func TestNewErrors(t *testing.T) {
	cases := map[string]struct {
		key, src string
		want     error
	}{
		"no dot":         {"pg", "x", ErrBadFormatKey},
		"empty format":   {"postgres.", "x", ErrBadFormatKey},
		"unknown engine": {"mysql.url", "x", ErrUnknownEngine},
		"bad template":   {"postgres.x", "{{.host", nil},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := New(map[string]string{tc.key: tc.src}, Bash)
			if err == nil || tc.want != nil && !errors.Is(err, tc.want) {
				t.Fatalf("err %v", err)
			}
		})
	}
}

func TestRenderMissingKey(t *testing.T) {
	r := newRenderer(t, map[string]string{"postgres.bad": "{{.nope}}"})
	if _, err := r.Render(Postgres, "bad", Conn{}); err == nil {
		t.Fatal("want error")
	}
}

func TestRenderCapsOutput(t *testing.T) {
	t.Parallel()
	src := `{{define "x"}}` + strings.Repeat("A", 500) + `{{template "x"}}{{end}}{{template "x"}}`
	if _, err := Preview(Postgres, "big", src, Bash); !errors.Is(err, ErrTooBig) {
		t.Fatalf("err %v", err)
	}
}

func TestRenderRefusesLongTemplate(t *testing.T) {
	t.Parallel()
	if _, err := Preview(Postgres, "long", strings.Repeat("x", MaxSource+1), Bash); !errors.Is(err, ErrTooLong) {
		t.Fatalf("err %v", err)
	}
}

func TestDotenvIsShellIndependent(t *testing.T) {
	pg := Conn{Host: "db", Port: "5432", Username: "bob", Password: "s3cret", Database: "app", Params: map[string]string{"sslmode": "require"}}
	cases := []struct {
		engine string
		conn   Conn
		want   string
	}{
		{Postgres, pg, "PGHOST=db\nPGPORT=5432\nPGUSER=bob\nPGPASSWORD=s3cret\nPGDATABASE=app\nPGSSLMODE=require"},
		{Postgres, Conn{Host: "db", Database: "app"}, "PGHOST=db\nPGDATABASE=app"},
		{Redis, Conn{Host: "cache", Port: "6380"}, "REDIS_URL=redis://cache:6380"},
		{Mongo, Conn{Host: "cluster", Database: "db"}, "MONGODB_URI=mongodb://cluster/db"},
	}
	for _, sh := range []Shell{Bash, Pwsh} {
		r, err := New(nil, sh)
		if err != nil {
			t.Fatal(err)
		}
		for _, c := range cases {
			got, err := r.Render(c.engine, "dotenv", c.conn)
			if err != nil {
				t.Fatal(err)
			}
			if got != c.want {
				t.Fatalf("%s %s.dotenv: got %q, want %q", sh, c.engine, got, c.want)
			}
		}
	}
}

func TestDotenvQuotesUnsafeValues(t *testing.T) {
	r := newRenderer(t, nil)
	c := Conn{Host: "db", Password: `a b"c$d\e` + "`f"}
	got, err := r.Render(Postgres, "dotenv", c)
	if err != nil {
		t.Fatal(err)
	}
	want := "PGHOST=db\n" + `PGPASSWORD="a b\"c\$d\\e\` + "`" + `f"`
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
