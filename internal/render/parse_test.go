package render

import (
	"errors"
	"reflect"
	"slices"
	"strings"
	"testing"
)

const tricky = `p@ss:w/rd'"`

func TestParse(t *testing.T) {
	cases := map[string]struct {
		engine, in string
		want       Conn
	}{
		"pg url": {Postgres, "postgresql://bob:p%40ss%3Aw%2Frd%27%22@db.local:5433/app?sslmode=require", Conn{
			Scheme: "postgresql", Host: "db.local", Port: "5433", Username: "bob", Password: tricky,
			Database: "app", Params: map[string]string{"sslmode": "require"},
		}},
		"pg url ipv6": {Postgres, "postgres://[::1]:5432/app", Conn{
			Scheme: "postgres", Host: "::1", Port: "5432", Database: "app", Params: map[string]string{},
		}},
		"pg url no host": {Postgres, "postgres:///app?host=/tmp", Conn{
			Scheme: "postgres", Database: "app", Params: map[string]string{"host": "/tmp"},
		}},
		"pg url multi host": {Postgres, "postgres://h1:1,h2:2/app", Conn{
			Scheme: "postgres", Host: "h1:1,h2:2", Database: "app", Params: map[string]string{},
		}},
		"pg url socket host": {Postgres, "postgres://%2Ftmp%20dir/app", Conn{
			Scheme: "postgres", Host: "/tmp dir", Database: "app", Params: map[string]string{},
		}},
		"mongo escaped multi": {Mongo, "mongodb://a%20b,[::1]/db", Conn{
			Scheme: "mongodb", Host: "a b,[::1]", Database: "db", Params: map[string]string{},
		}},
		"pg dsn": {Postgres, ` host=db.local port = 5432 user=bob password='p@ss:w/rd\'"' dbname=app sslmode=disable `, Conn{
			Host: "db.local", Port: "5432", Username: "bob", Password: tricky,
			Database: "app", Params: map[string]string{"sslmode": "disable"},
		}},
		"pg dsn escapes": {Postgres, `password=a\ b\\c dbname=''`, Conn{
			Password: `a b\c`, Params: map[string]string{},
		}},
		"redis": {Redis, "redis://:secret@cache:6380/2", Conn{
			Scheme: "redis", Host: "cache", Port: "6380", Password: "secret", Database: "2", Params: map[string]string{},
		}},
		"rediss user": {Redis, "rediss://u:p@cache", Conn{
			Scheme: "rediss", Host: "cache", Username: "u", Password: "p", Params: map[string]string{},
		}},
		"mongo multi": {Mongo, "mongodb://u:p@h1:27017,h2:27018/app?replicaSet=rs0&authSource=admin", Conn{
			Scheme: "mongodb", Host: "h1:27017,h2:27018", Username: "u", Password: "p", Database: "app",
			Params: map[string]string{"replicaSet": "rs0", "authSource": "admin"},
		}},
		"mongo srv": {Mongo, "mongodb+srv://u:p@cluster.example.com/?retryWrites=true", Conn{
			Scheme: "mongodb+srv", Host: "cluster.example.com", Username: "u", Password: "p",
			Params: map[string]string{"retryWrites": "true"},
		}},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := Parse(tc.engine, tc.in)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("got %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestParseInvalid(t *testing.T) {
	cases := map[string]struct{ engine, in string }{
		"pg empty":         {Postgres, "  "},
		"pg bad scheme":    {Postgres, "mysql://h/db"},
		"pg bad port":      {Postgres, "postgres://h:99999/db"},
		"pg port text":     {Postgres, "postgres://h:abc/db"},
		"pg ipv6 bare":     {Postgres, "postgres://::1:5432/db"},
		"pg fragment":      {Postgres, "postgres://h/db#x"},
		"pg bad escape":    {Postgres, "postgres://u:%zz@h/db"},
		"pg bad query":     {Postgres, "postgres://h/db?a=%zz"},
		"pg host escape":   {Postgres, "postgres://h%zz/db"},
		"pg host comma":    {Postgres, "postgres://a%2Cb/db"},
		"mongo host brace": {Mongo, "mongodb://a%5Bb/db"},
		"pg empty in list": {Postgres, "postgres://:,0"},
		"pg mongo url":     {Postgres, "mongodb://u:p@host/db?retryWrites=true"},
		"pg redis url":     {Postgres, "rediss://cache:6379/0?x=1"},
		"pg mysql url":     {Postgres, "mysql://u:p@h/db?a=b"},
		"pg url no equals": {Postgres, "redis://cache:6379/0"},
		"dsn no value":     {Postgres, "host"},
		"dsn open quote":   {Postgres, "password='abc"},
		"dsn after quote":  {Postgres, "password='a'b"},
		"dsn trailing esc": {Postgres, `password=a\`},
		"dsn bad port":     {Postgres, "port=0"},
		"redis no host":    {Redis, "redis:///0"},
		"redis multi":      {Redis, "redis://a,b"},
		"redis db":         {Redis, "redis://h/x"},
		"redis scheme":     {Redis, "http://h"},
		"mongo no host":    {Mongo, "mongodb:///db"},
		"mongo srv port":   {Mongo, "mongodb+srv://h:1/db"},
		"mongo srv multi":  {Mongo, "mongodb+srv://a,b/db"},
		"mongo empty host": {Mongo, "mongodb://a,,b/db"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := Parse(tc.engine, tc.in); !errors.Is(err, ErrInvalidConn) {
				t.Fatalf("err %v", err)
			}
		})
	}
}

func TestParseUnknownEngine(t *testing.T) {
	if _, err := Parse("mysql", "x"); !errors.Is(err, ErrUnknownEngine) {
		t.Fatalf("err %v", err)
	}
}

func TestParseErrorHidesSecret(t *testing.T) {
	for _, in := range []string{"password='hunter2", "postgres://u:hunter2@h:x/db"} {
		_, err := Parse(Postgres, in)
		if err == nil || strings.Contains(err.Error(), "hunter2") {
			t.Fatalf("err %v", err)
		}
	}
}

func TestParsePostgresRefusesOtherEngineURL(t *testing.T) {
	for _, in := range []string{
		"mongodb://u:p@host/db?retryWrites=true",
		"mongodb+srv://u:p@cluster/db?retryWrites=true",
		"rediss://cache:6379/0?x=1",
		"  redis://cache:6379/0?a=b",
	} {
		got, err := Parse(Postgres, in)
		if !errors.Is(err, ErrInvalidConn) {
			t.Fatalf("%q: err %v", in, err)
		}
		if !reflect.DeepEqual(got, Conn{}) {
			t.Fatalf("%q: conn %+v", in, got)
		}
	}
}

func TestParsePostgresDSNKeepsValueWithScheme(t *testing.T) {
	got, err := Parse(Postgres, "host=db options=x://y")
	if err != nil {
		t.Fatal(err)
	}
	if got.Host != "db" || got.Params["options"] != "x://y" {
		t.Fatalf("conn %+v", got)
	}
}

func TestEngineFor(t *testing.T) {
	cases := map[string]struct {
		in     string
		engine string
		ok     bool
	}{
		"postgres":     {"postgres://h/db", Postgres, true},
		"postgresql":   {"postgresql://h/db", Postgres, true},
		"redis":        {"redis://h/0", Redis, true},
		"rediss":       {"rediss://h/0", Redis, true},
		"mongodb":      {"mongodb://h/db", Mongo, true},
		"mongodb srv":  {"mongodb+srv://c/db", Mongo, true},
		"leading ws":   {"  redis://h/0", Redis, true},
		"dsn host":     {"host=db dbname=app", Postgres, true},
		"dsn port":     {"port=5432", Postgres, true},
		"dsn user":     {"user=bob", Postgres, true},
		"random pair":  {"a=b", "", false},
		"empty":        {"", "", false},
		"other scheme": {"ftp://x", "", false},
		"mysql":        {"mysql://u:p@h/db", "", false},
		"junk":         {"hello world", "", false},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			engine, ok := EngineFor(tc.in)
			if engine != tc.engine || ok != tc.ok {
				t.Fatalf("got %q %v, want %q %v", engine, ok, tc.engine, tc.ok)
			}
		})
	}
}

func TestEngineForAgreesWithParse(t *testing.T) {
	for engine, schemes := range engineSchemes {
		for _, scheme := range schemes {
			db, host := "db", "host:1234"
			if engine == Redis {
				db = "0"
			}
			if scheme == "mongodb+srv" {
				host = "cluster.example.com"
			}
			in := scheme + "://user:pw@" + host + "/" + db
			got, ok := EngineFor(in)
			if !ok || got != engine {
				t.Fatalf("%q: got %q %v, want %q", in, got, ok, engine)
			}
			if _, err := Parse(engine, in); err != nil {
				t.Fatalf("%q: parse %v", in, err)
			}
		}
	}
}

func TestDefaultSchemeIsAKnownScheme(t *testing.T) {
	for engine, scheme := range defaultScheme {
		if !slices.Contains(engineSchemes[engine], scheme) {
			t.Fatalf("%s default scheme %q is not in %q", engine, scheme, engineSchemes[engine])
		}
	}
	if len(defaultScheme) != len(engineSchemes) {
		t.Fatalf("%d default schemes for %d engines", len(defaultScheme), len(engineSchemes))
	}
}
