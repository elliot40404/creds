package envfile

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

func parseString(t *testing.T, s string) ([]Var, []string) {
	t.Helper()
	vars, warns, err := Parse(strings.NewReader(s))
	if err != nil {
		t.Fatalf("parse %q: %v", s, err)
	}
	return vars, warns
}

func TestParseForms(t *testing.T) {
	src := "# top comment\n" +
		"\n" +
		"A=plain\n" +
		"export B=exported\n" +
		"  C = spaced  \n" +
		"D=\n" +
		"E= # only comment\n" +
		"F=a#b\n" +
		"G=a #trailing\n" +
		"H='single #x \\n'  # c\n" +
		"I=\"dq \\n \\t \\\\ \\\" \\$ \\` \\q\"\n" +
		"J=a=b\n" +
		"K=#notcomment\n" +
		"L='multi\nline'\n" +
		"M=\"two\nlines\"\n" +
		"export=kw\r\n" +
		"N=crlf\r\n" +
		"O=last"
	want := []Var{
		{"A", "plain"},
		{"B", "exported"},
		{"C", "spaced"},
		{"D", ""},
		{"E", ""},
		{"F", "a#b"},
		{"G", "a"},
		{"H", "single #x \\n"},
		{"I", "dq \n \t \\ \" $ ` \\q"},
		{"J", "a=b"},
		{"K", "#notcomment"},
		{"L", "multi\nline"},
		{"M", "two\nlines"},
		{"export", "kw"},
		{"N", "crlf"},
		{"O", "last"},
	}
	got, warns := parseString(t, src)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got  %q\nwant %q", got, want)
	}
	if len(warns) != 0 {
		t.Fatalf("warns %v", warns)
	}
}

func TestParseDuplicateLastWins(t *testing.T) {
	got, warns := parseString(t, "A=1\nB=2\nA=3\n")
	want := []Var{{"A", "3"}, {"B", "2"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %q", got)
	}
	if len(warns) != 1 || !strings.Contains(warns[0], "line 3") || !strings.Contains(warns[0], "A") {
		t.Fatalf("warns %v", warns)
	}
}

func TestParseErrors(t *testing.T) {
	cases := []struct {
		src  string
		want error
		line string
	}{
		{"1A=x", ErrBadKey, "line 1"},
		{"OK=1\nFOO-BAR=x", ErrBadKey, "line 2"},
		{"=x", ErrBadKey, "line 1"},
		{"FOO BAR=x", ErrBadKey, "line 1"},
		{"OK=1\nBAD KEY=2", ErrBadKey, "line 2"},
		{"FOO bar", ErrSyntax, "line 1"},
		{"FOO", ErrSyntax, "line 1"},
		{"A='open", ErrSyntax, "line 1"},
		{"A=\"open", ErrSyntax, "line 1"},
		{"A=\"open\\", ErrSyntax, "line 1"},
		{"A='x' junk", ErrSyntax, "line 1"},
		{"A=\"x\"junk", ErrSyntax, "line 1"},
		{"A='a\nb'\nB=\"c\nd\"\n!=x", ErrBadKey, "line 5"},
		{"A=x\x00", ErrNUL, ""},
	}
	for _, c := range cases {
		_, _, err := Parse(strings.NewReader(c.src))
		if !errors.Is(err, c.want) || !strings.Contains(err.Error(), c.line) {
			t.Errorf("%q: got %v want %v at %s", c.src, err, c.want, c.line)
		}
	}
}

func TestParseErrorsHideText(t *testing.T) {
	for _, src := range []string{"A=1\nghp_SECRET123", "A=1\nMIIEvQIB+more", "SECRETKEY value"} {
		_, _, err := Parse(strings.NewReader(src))
		if err == nil || strings.Contains(err.Error(), "SECRET") || strings.Contains(err.Error(), "MIIE") {
			t.Errorf("%q: got %v", src, err)
		}
	}
}

func TestParseTooLarge(t *testing.T) {
	src := "A=" + strings.Repeat("x", maxSize)
	if _, _, err := Parse(strings.NewReader(src)); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("got %v", err)
	}
}
