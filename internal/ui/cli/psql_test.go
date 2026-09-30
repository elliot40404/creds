package cli

import "testing"

func TestAddAndCopyPsqlCommand(t *testing.T) {
	t.Parallel()
	h := seeded(t)
	h.writeConfig("[render]\nshell = \"pwsh\"\n")
	command := "psql 'host=db.example port=5432 user=bob password=" + dbHidden + " dbname=postgres sslnegotiation=direct sslmode=verify-full sslrootcert=system'"
	r := h.ok(&fake{passwords: []string{command}, inputs: []string{""}}, "add", "--type", "database", "db/psql")
	noLeak(t, r, dbHidden)
	uri := "postgresql://bob:" + dbHidden + "@db.example:5432/postgres?sslmode=verify-full&sslnegotiation=direct&sslrootcert=system"
	for _, format := range []string{"psql", "pg_dump", "pg_restore"} {
		r := h.ok(&fake{}, "copy", "db/psql", "--as", format)
		noLeak(t, r, dbHidden)
		prefix := format + " "
		if format == "pg_restore" {
			prefix += "-d "
		}
		if want := prefix + "'" + uri + "'"; h.clip.value != want {
			t.Fatalf("%s copied %q, want %q", format, h.clip.value, want)
		}
	}
}
