package envfile

import (
	"bytes"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestWriteForms(t *testing.T) {
	vars := []Var{
		{"EMPTY", ""},
		{"BARE", "host.example:5432/db-1"},
		{"SPACE", "a b"},
		{"HASH", "#x"},
		{"QUOTE", "it's"},
		{"NL", "a\nb"},
		{"SHELL", "$HOME `x` \\ \"q\" 'r'"},
	}
	var buf bytes.Buffer
	if err := Write(&buf, vars); err != nil {
		t.Fatal(err)
	}
	want := "EMPTY=\n" +
		"BARE=host.example:5432/db-1\n" +
		"SPACE='a b'\n" +
		"HASH='#x'\n" +
		"QUOTE=\"it's\"\n" +
		"NL=\"a\\nb\"\n" +
		"SHELL=\"\\$HOME \\`x\\` \\\\ \\\"q\\\" 'r'\"\n"
	if buf.String() != want {
		t.Fatalf("got\n%s\nwant\n%s", buf.String(), want)
	}
}

func TestWriteRoundtrip(t *testing.T) {
	vars := []Var{
		{"A", ""},
		{"B", "plain"},
		{"C", "  lead and trail  "},
		{"D", "line1\nline2\r\nline3\r"},
		{"E", `both ' and " quotes`},
		{"F", "a=b=c"},
		{"G", "x # not a comment"},
		{"H", "#start"},
		{"I", `back\slash\n literal`},
		{"J", "tab\there"},
		{"K", "$VAR ${VAR} `cmd`"},
		{"L", "\n"},
		{"M", "'"},
		{"N", "\""},
		{"O", "\\"},
		{"P", "ünïcödé"},
		{"export", "kw"},
	}
	var buf bytes.Buffer
	if err := Write(&buf, vars); err != nil {
		t.Fatal(err)
	}
	got, warns, err := Parse(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, vars) {
		t.Fatalf("got  %q\nwant %q", got, vars)
	}
	if len(warns) != 0 {
		t.Fatalf("warns %v", warns)
	}
}

func TestWriteErrors(t *testing.T) {
	cases := []struct {
		vars []Var
		want error
	}{
		{[]Var{{"1A", "x"}}, ErrBadKey},
		{[]Var{{"", "x"}}, ErrBadKey},
		{[]Var{{"A B", "x"}}, ErrBadKey},
		{[]Var{{"A", "x"}, {"A", "y"}}, ErrDuplicateKey},
		{[]Var{{"A", "x\x00"}}, ErrNUL},
	}
	for _, c := range cases {
		var buf bytes.Buffer
		if err := Write(&buf, c.vars); !errors.Is(err, c.want) {
			t.Errorf("%q: got %v want %v", c.vars, err, c.want)
		}
		if buf.Len() != 0 {
			t.Errorf("%q: partial output %q", c.vars, buf.String())
		}
	}
}

func FuzzRoundtrip(f *testing.F) {
	for _, s := range []string{"", "a", "a b", "'\"\\\n\r$`#="} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, v string) {
		if strings.IndexByte(v, 0) >= 0 {
			return
		}
		vars := []Var{{"K", v}}
		var buf bytes.Buffer
		if err := Write(&buf, vars); err != nil {
			t.Fatal(err)
		}
		got, _, err := Parse(&buf)
		if err != nil {
			t.Fatalf("%q: %v", buf.String(), err)
		}
		if !reflect.DeepEqual(got, vars) {
			t.Fatalf("got %q want %q", got, vars)
		}
	})
}
