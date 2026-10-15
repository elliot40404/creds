package textdiff

import (
	"strings"
	"testing"
)

func render(lines []Line) string {
	var b strings.Builder
	for _, l := range lines {
		b.WriteByte(byte(l.Op))
		b.WriteString(l.Text)
		b.WriteByte('\n')
	}
	return b.String()
}

func TestLines(t *testing.T) {
	t.Parallel()
	cases := []struct{ a, b, want string }{
		{"", "", ""},
		{"a\n", "a\n", " a\n"},
		{"", "a\nb\n", "+a\n+b\n"},
		{"a\nb\n", "", "-a\n-b\n"},
		{"a\nb\nc\n", "a\nx\nc\n", " a\n-b\n+x\n c\n"},
		{"a\nb\nc\nd\n", "a\nc\nd\ne\n", " a\n-b\n c\n d\n+e\n"},
		{"x\na\ny\nb\n", "a\nb\nz\n", "-x\n a\n-y\n b\n+z\n"},
		{"a", "a\n", " a\n"},
	}
	for _, c := range cases {
		got := Lines(c.a, c.b)
		if render(got) != c.want {
			t.Errorf("%q -> %q: got %q, want %q", c.a, c.b, render(got), c.want)
		}
		changed := strings.Contains("\n"+c.want, "\n+") || strings.Contains("\n"+c.want, "\n-")
		if Changed(got) != changed {
			t.Errorf("%q -> %q: changed %v", c.a, c.b, Changed(got))
		}
	}
}

func TestLinesFallsBackOnHugeInput(t *testing.T) {
	t.Parallel()
	a := strings.Repeat("a\n", 2000)
	b := strings.Repeat("b\n", 2000)
	got := Lines("head\n"+a+"tail\n", "head\n"+b+"tail\n")
	if len(got) != 4002 || got[0].Op != Same || got[1].Op != Del || got[2001].Op != Add || got[4001].Op != Same {
		t.Fatalf("got %d lines", len(got))
	}
}

func TestContext(t *testing.T) {
	t.Parallel()
	a := "1\n2\n3\n4\n5\n6\n7\n8\n9\n"
	b := "1\n2\n3\n4\nx\n6\n7\n8\n9\n"
	got := render(Context(Lines(a, b), 1))
	if got != "~\n 4\n-5\n+x\n 6\n~\n" {
		t.Fatalf("got %q", got)
	}
	if got := render(Context(Lines(a, a), 1)); got != "~\n" {
		t.Fatalf("same %q", got)
	}
}
