package render

import (
	"errors"
	"slices"
	"strings"
	"testing"
)

func TestPwshQuote(t *testing.T) {
	cases := map[string]string{
		"":              "''",
		"plain":         "'plain'",
		"db.local:5432": "'db.local:5432'",
		"a b":           "'a b'",
		tricky:          `'p@ss:w/rd''"'`,
		"$HOME`x`":      "'$HOME`x`'",
		"a;rm -rf /":    "'a;rm -rf /'",
		"--%":           "'--%'",
		"@args":         "'@args'",
		"a,b":           "'a,b'",
		"x’y":           "'x’’y'",
		"‘q‛":           "'‘‘q‛‛'",
		"$(calc)":       "'$(calc)'",
		"line\nbreak":   "'line\nbreak'",
	}
	for in, want := range cases {
		if got := pwshQuote(in); got != want {
			t.Errorf("pwshQuote(%q) = %s, want %s", in, got, want)
		}
	}
}

func TestParseShell(t *testing.T) {
	for in, want := range map[string]Shell{"": DefaultShell(), "bash": Bash, "pwsh": Pwsh} {
		got, err := ParseShell(in)
		if err != nil || got != want {
			t.Fatalf("ParseShell(%q) = %q, %v", in, got, err)
		}
	}
	if _, err := ParseShell("fish"); !errors.Is(err, ErrUnknownShell) {
		t.Fatalf("err %v", err)
	}
	if _, err := New(nil, "fish"); !errors.Is(err, ErrUnknownShell) {
		t.Fatalf("New err %v", err)
	}
}

func TestRenderPwsh(t *testing.T) {
	r, err := New(nil, Pwsh)
	if err != nil {
		t.Fatal(err)
	}
	pg := Conn{Host: "db.local", Port: "5432", Username: "bob", Password: tricky, Database: "app", Params: map[string]string{"sslmode": "require"}}
	rd := Conn{Scheme: "rediss", Host: "cache", Username: "u", Password: "it's"}
	mg := Conn{Host: "h", Params: map[string]string{"a": "1", "b": "2"}}
	quoted := pwshQuote(tricky)
	cases := []struct {
		engine, format string
		c              Conn
		want           string
	}{
		{Postgres, "psql", pg, "$env:PGPASSWORD=" + quoted + "; $env:PGSSLMODE='require'; psql -h 'db.local' -p '5432' -U 'bob' -d 'app'; Remove-Item Env:PGPASSWORD; Remove-Item Env:PGSSLMODE"},
		{Postgres, "psql", Conn{Host: "h"}, "psql -h 'h'"},
		{Postgres, "env", pg, "$env:PGHOST='db.local'\n$env:PGPORT='5432'\n$env:PGUSER='bob'\n$env:PGPASSWORD=" + quoted + "\n$env:PGDATABASE='app'\n$env:PGSSLMODE='require'"},
		{Redis, "redis-cli", rd, "$env:REDISCLI_AUTH='it''s'; redis-cli -h 'cache' --user 'u' --tls; Remove-Item Env:REDISCLI_AUTH"},
		{Redis, "env", Conn{Host: "cache"}, "$env:REDIS_URL='redis://cache'"},
		{Mongo, "mongosh", mg, "mongosh 'mongodb://h/?a=1&b=2'"},
		{Mongo, "mongodump", mg, "mongodump '--uri=mongodb://h/?a=1&b=2'"},
		{Mongo, "env", mg, "$env:MONGODB_URI='mongodb://h/?a=1&b=2'"},
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

func TestShellQuote(t *testing.T) {
	cases := map[string]string{
		"":              "''",
		"plain":         "plain",
		"db.local:5432": "db.local:5432",
		"a b":           "'a b'",
		tricky:          `'p@ss:w/rd'\''"'`,
		"$HOME`x`":      "'$HOME`x`'",
		"a;rm -rf /":    "'a;rm -rf /'",
	}
	for in, want := range cases {
		if got := shellQuote(in); got != want {
			t.Errorf("shellQuote(%q) = %s, want %s", in, got, want)
		}
	}
}

func TestShellQuoteWords(t *testing.T) {
	t.Parallel()
	values := []string{"web/app", "", "a b", "x';Write-Output INJECTED;'", "it’s", "a,b", "@x", "$(calc)", "`n", "k=v", "-x"}
	for _, v := range values {
		q := Pwsh.Quote(v)
		if q == v {
			if strings.ContainsAny(v, "',@$`; ") || v == "" {
				t.Errorf("pwsh left %q bare", v)
			}
			continue
		}
		if lits, err := pwshLiterals("psql " + q); err != nil || !slices.Equal(lits, []string{v}) {
			t.Errorf("pwsh %q -> %q: %q %v", v, q, lits, err)
		}
	}
	if got := Bash.Quote("it's"); got != `'it'\''s'` {
		t.Errorf("bash %s", got)
	}
}
