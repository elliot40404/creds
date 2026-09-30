package render

import (
	"reflect"
	"strings"
	"testing"
)

func TestPostgresCommandsPreserveConnection(t *testing.T) {
	in := "postgresql://bob:" + trickyEsc + "@db.local:5432/app?application_name=copy%20check&connect_timeout=5&sslmode=require&sslrootcert=C%3A%2FProgram%20Files%2Fcert.pem"
	c, err := Parse(Postgres, in)
	if err != nil {
		t.Fatal(err)
	}
	for _, shell := range []Shell{Bash, Pwsh} {
		r, err := New(nil, shell)
		if err != nil {
			t.Fatal(err)
		}
		for _, command := range []string{"psql", "pg_dump", "pg_restore"} {
			t.Run(string(shell)+"/"+command, func(t *testing.T) {
				prefix := command + " "
				if command == "pg_restore" {
					prefix += "-d "
				}
				got, err := r.Render(Postgres, command, c)
				if err != nil {
					t.Fatal(err)
				}
				if want := prefix + "'" + in + "'"; got != want {
					t.Fatalf("got %q, want %q", got, want)
				}
				if shell == Pwsh {
					literals, err := pwshLiterals(got)
					if err != nil || len(literals) != 1 {
						t.Fatalf("literals %q, err %v", literals, err)
					}
					parsed, err := Parse(Postgres, literals[0])
					if err != nil || !reflect.DeepEqual(parsed, c) {
						t.Fatalf("connection changed: %+v, err %v", parsed, err)
					}
				}
				if strings.Contains(got, "PGPASSWORD") {
					t.Fatal("command changes password environment")
				}
			})
		}
	}
}

func TestPostgresDSNCommandsPreserveSSL(t *testing.T) {
	c, err := Parse(Postgres, "host=db.example port=5432 user=bob password=test-password dbname=postgres sslnegotiation=direct sslmode=verify-full sslrootcert=system")
	if err != nil {
		t.Fatal(err)
	}
	r, err := New(nil, Pwsh)
	if err != nil {
		t.Fatal(err)
	}
	for _, format := range []string{"psql", "pg_dump", "pg_restore"} {
		command, err := r.Render(Postgres, format, c)
		if err != nil {
			t.Fatal(err)
		}
		literals, err := pwshLiterals(command)
		if err != nil || len(literals) != 1 {
			t.Fatalf("literals %q, err %v", literals, err)
		}
		parsed, err := Parse(Postgres, literals[0])
		if err != nil {
			t.Fatal(err)
		}
		parsed.Scheme = c.Scheme
		if !reflect.DeepEqual(parsed, c) {
			t.Fatalf("%s changed connection: %+v", format, parsed)
		}
	}
}
