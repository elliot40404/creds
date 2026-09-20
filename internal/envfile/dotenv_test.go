package envfile

import (
	"bytes"
	"errors"
	"regexp"
	"strings"
	"testing"
)

var (
	pyEscape = regexp.MustCompile(`\\[\\'"abfnrtv]`)
	pyExpand = regexp.MustCompile(`\$\{[^}]*\}`)
)

func nodeDotenv(raw string) string {
	if len(raw) >= 2 && raw[0] == raw[len(raw)-1] && strings.ContainsRune(`'"`, rune(raw[0])) {
		inner := raw[1 : len(raw)-1]
		if raw[0] == '"' {
			inner = strings.NewReplacer(`\n`, "\n", `\r`, "\r").Replace(inner)
		}
		return inner
	}
	return strings.TrimSpace(raw)
}

func pythonDotenv(raw string) string {
	switch {
	case len(raw) >= 2 && raw[0] == '\'' && raw[len(raw)-1] == '\'':
		return strings.NewReplacer(`\\`, `\`, `\'`, `'`).Replace(raw[1 : len(raw)-1])
	case len(raw) >= 2 && raw[0] == '"' && raw[len(raw)-1] == '"':
		inner := pyEscape.ReplaceAllStringFunc(raw[1:len(raw)-1], func(e string) string {
			return map[byte]string{'\\': `\`, '\'': `'`, '"': `"`, 'a': "\a", 'b': "\b", 'f': "\f", 'n': "\n", 'r': "\r", 't': "\t", 'v': "\v"}[e[1]]
		})
		return pyExpand.ReplaceAllString(inner, "")
	}
	return pyExpand.ReplaceAllString(strings.TrimSpace(raw), "")
}

func dotenvLine(t *testing.T, value string) (string, error) {
	t.Helper()
	var buf bytes.Buffer
	err := WriteDotenv(&buf, []Var{{"K", value}})
	return strings.TrimSuffix(buf.String(), "\n"), err
}

func TestWriteDotenvReadsBackInNodeAndPython(t *testing.T) {
	cases := []struct{ value, line string }{
		{"", "K="},
		{"host.example:5432/db-1", "K=host.example:5432/db-1"},
		{"it's$ecret", `K="it's$ecret"`},
		{"p@ss word", "K='p@ss word'"},
		{`say "hi"`, `K='say "hi"'`},
		{"${HOME}", "K='${HOME}'"},
		{`C:\Users\me`, `K='C:\Users\me'`},
		{"two\nlines", `K="two\nlines"`},
		{"#not a comment", "K='#not a comment'"},
	}
	for _, c := range cases {
		line, err := dotenvLine(t, c.value)
		if err != nil || line != c.line {
			t.Errorf("%q: got %q %v, want %q", c.value, line, err, c.line)
			continue
		}
		raw := strings.TrimPrefix(line, "K=")
		if got := nodeDotenv(raw); got != c.value {
			t.Errorf("%q: node reads %q", c.value, got)
		}
		if got := pythonDotenv(raw); got != c.value {
			t.Errorf("%q: python reads %q", c.value, got)
		}
	}
}

func TestWriteDotenvRefusesWhatItCannotWrite(t *testing.T) {
	for _, v := range []string{`it's "both"`, "it's ${HOME}", `it's a\b`, "trail\\", `a\\b`, "it's\r\nx\\"} {
		if line, err := dotenvLine(t, v); !errors.Is(err, ErrNotDotenv) {
			t.Errorf("%q: got %q %v", v, line, err)
		}
	}
}
