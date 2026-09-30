package render

import (
	"errors"
	"testing"
)

func TestParsePsqlCommand(t *testing.T) {
	inputs := []string{
		`psql 'host=h port=5432 user=bob password=p dbname=postgres sslnegotiation=direct sslmode=verify-full sslrootcert=system'`,
		`psql "host=h port=5432 user=bob password=p dbname=postgres sslnegotiation=direct sslmode=verify-full sslrootcert=system"`,
	}
	for _, in := range inputs {
		engine, ok := EngineFor(in)
		if !ok || engine != Postgres {
			t.Fatalf("engine %q, ok %v", engine, ok)
		}
		c, err := Parse(engine, in)
		if err != nil {
			t.Fatal(err)
		}
		if c.Host != "h" || c.Port != "5432" || c.Username != "bob" || c.Password != "p" || c.Database != "postgres" {
			t.Fatalf("connection changed: %+v", c)
		}
		for key, want := range map[string]string{"sslnegotiation": "direct", "sslmode": "verify-full", "sslrootcert": "system"} {
			if c.Params[key] != want {
				t.Fatalf("%s = %q, want %q", key, c.Params[key], want)
			}
		}
	}
}

func TestParsePsqlQuotedArgument(t *testing.T) {
	for _, in := range []string{
		`psql 'postgresql://h/db''name'`,
		`psql 'postgresql://h/db'\''name'`,
		`psql "postgresql://h/db'name"`,
	} {
		c, err := Parse(Postgres, in)
		if err != nil || c.Database != "db'name" {
			t.Fatalf("%q: connection %+v, err %v", in, c, err)
		}
	}
	c, err := Parse(Postgres, `psql "host=h password='a b'"`)
	if err != nil || c.Password != "a b" {
		t.Fatalf("connection %+v, err %v", c, err)
	}
	c, err = Parse(Postgres, "psql postgresql://h/app")
	if err != nil || c.Database != "app" {
		t.Fatalf("connection %+v, err %v", c, err)
	}
}

func TestParsePsqlRejectsExtraCommands(t *testing.T) {
	for _, in := range []string{
		"psql", "psql ''", "psql 'host=h", "psql 'redis://h'",
		"psql 'host=h'; calc", "psql 'host=h' --password", "psql host=h dbname=app",
	} {
		if _, err := Parse(Postgres, in); !errors.Is(err, ErrInvalidConn) {
			t.Fatalf("%q: err %v", in, err)
		}
		if engine, ok := EngineFor(in); ok {
			t.Fatalf("%q: engine %q", in, engine)
		}
	}
}

func TestPsqlInputPreservesDSNEscapes(t *testing.T) {
	c, err := Parse(Postgres, `host=h password=tail\ `)
	if err != nil || c.Password != "tail " {
		t.Fatalf("connection %+v, err %v", c, err)
	}
}
