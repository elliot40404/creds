package render

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"testing"
)

type pwshToken struct {
	text   string
	quoted bool
	glued  bool
}

var (
	pwshAssign  = regexp.MustCompile(`^\$env:[A-Z_]+=$`)
	pwshEnvName = regexp.MustCompile(`^Env:[A-Z_]+$`)
	pwshFlag    = regexp.MustCompile(`^--?[a-zA-Z]+$`)
	pwshCmds    = []string{"psql", "pg_dump", "pg_restore", "redis-cli", "mongosh", "mongodump", "mongorestore"}
	errPwsh     = errors.New("pwsh line not literal")
)

func pwshQuoted(rs []rune, i int) (string, int, error) {
	var b strings.Builder
	for i++; i < len(rs); i++ {
		if !isPwshSingleQuote(rs[i]) {
			b.WriteRune(rs[i])
			continue
		}
		if i+1 < len(rs) && isPwshSingleQuote(rs[i+1]) {
			i++
			b.WriteRune(rs[i])
			continue
		}
		return b.String(), i + 1, nil
	}
	return "", 0, fmt.Errorf("%w: unterminated quote", errPwsh)
}

func pwshTokens(s string) ([][]pwshToken, error) {
	var stmts [][]pwshToken
	var cur []pwshToken
	rs := []rune(s)
	end := -1
	for i := 0; i < len(rs); {
		switch r := rs[i]; {
		case r == ' ':
			i++
		case r == ';' || r == '\n':
			stmts, cur = append(stmts, cur), nil
			i++
		case isPwshSingleQuote(r):
			text, next, err := pwshQuoted(rs, i)
			if err != nil {
				return nil, err
			}
			cur = append(cur, pwshToken{text: text, quoted: true, glued: end == i})
			i, end = next, next
		default:
			j := i
			for j < len(rs) && !strings.ContainsRune(" ;\n", rs[j]) && !isPwshSingleQuote(rs[j]) {
				j++
			}
			cur = append(cur, pwshToken{text: string(rs[i:j]), glued: end == i})
			i, end = j, j
		}
	}
	return append(stmts, cur), nil
}

func pwshLiterals(s string) ([]string, error) {
	stmts, err := pwshTokens(s)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, st := range stmts {
		lits, err := pwshStatement(st)
		if err != nil {
			return nil, fmt.Errorf("%w in %q", err, s)
		}
		out = append(out, lits...)
	}
	return out, nil
}

func pwshStatement(st []pwshToken) ([]string, error) {
	if len(st) == 0 || st[0].quoted {
		return nil, fmt.Errorf("%w: bad statement", errPwsh)
	}
	head := st[0].text
	switch {
	case pwshAssign.MatchString(head):
		if len(st) != 2 || !st[1].quoted || !st[1].glued {
			return nil, fmt.Errorf("%w: assignment value not quoted", errPwsh)
		}
		return []string{st[1].text}, nil
	case head == "Remove-Item":
		if len(st) != 2 || st[1].quoted || !pwshEnvName.MatchString(st[1].text) {
			return nil, fmt.Errorf("%w: bad Remove-Item", errPwsh)
		}
		return nil, nil
	case slices.Contains(pwshCmds, head):
		var out []string
		for _, tok := range st[1:] {
			switch {
			case tok.quoted && !tok.glued:
				out = append(out, tok.text)
			case tok.quoted || tok.glued || !pwshFlag.MatchString(tok.text):
				return nil, fmt.Errorf("%w: bare word %q", errPwsh, tok.text)
			}
		}
		return out, nil
	}
	return nil, fmt.Errorf("%w: unknown statement %q", errPwsh, head)
}

func checkPwsh(t *testing.T, r *Renderer, engine string, c Conn) {
	t.Helper()
	formats, err := r.Formats(engine)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range formats {
		if name == FormatURL || name == "dotenv" {
			continue
		}
		out, err := r.Render(engine, name, c)
		if err != nil {
			t.Fatal(err)
		}
		if out == "" {
			continue
		}
		if _, err := pwshLiterals(out); err != nil {
			t.Fatalf("%s.%s: %v", engine, name, err)
		}
	}
}

func wantLiterals(t *testing.T, out string, n int, v string) {
	t.Helper()
	lits, err := pwshLiterals(out)
	if err != nil {
		t.Fatal(err)
	}
	if len(lits) != n {
		t.Fatalf("got %d literals %q, want %d", len(lits), lits, n)
	}
	for _, got := range lits {
		if got != v {
			t.Fatalf("literal %q, want %q in %q", got, v, out)
		}
	}
}

func TestPwshRenderedLinesAreLiteral(t *testing.T) {
	r, err := New(nil, Pwsh)
	if err != nil {
		t.Fatal(err)
	}
	evil := []string{"db.example.com", "calc", "$(calc)", "a;calc", "x’;calc;’", "`n", "@calc", "5432", "it's", "a\nb", "require&calc"}
	for _, v := range evil {
		pg := Conn{Host: v, Port: v, Username: v, Password: v, Database: v, Params: map[string]string{"sslmode": v}}
		want := map[string]int{"env": 6, "psql": 6, "pg_dump": 6, "pg_restore": 6}
		for format, n := range want {
			out, err := r.Render(Postgres, format, pg)
			if err != nil {
				t.Fatal(err)
			}
			wantLiterals(t, out, n, v)
		}
		rd := Conn{Host: v, Port: v, Username: v, Password: v, Database: v}
		out, err := r.Render(Redis, "redis-cli", rd)
		if err != nil {
			t.Fatal(err)
		}
		wantLiterals(t, out, 5, v)
		mg := Conn{Host: "h", Params: map[string]string{"sslmode": v}}
		checkPwsh(t, r, Redis, mg)
		checkPwsh(t, r, Mongo, mg)
	}
}

func TestPwshCheckerRejectsBareWords(t *testing.T) {
	bad := []string{
		"$env:PGHOST=db.example.com",
		"$env:PGHOST= 'x'",
		"psql -h db.local",
		"psql -h 'a'b",
		"psql -h 'a'-x",
		"calc",
		"Remove-Item Env:X; calc",
		"$env:X='open",
	}
	for _, s := range bad {
		if _, err := pwshLiterals(s); !errors.Is(err, errPwsh) {
			t.Errorf("%q accepted", s)
		}
	}
	lits, err := pwshLiterals("$env:PGHOST='db.example.com'\npsql -h 'it''s'; Remove-Item Env:PGHOST")
	if err != nil || !slices.Equal(lits, []string{"db.example.com", "it's"}) {
		t.Fatalf("lits %q err %v", lits, err)
	}
}
