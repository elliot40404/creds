package safetext

import (
	"testing"
)

var (
	csi  = string(rune(0x9b))
	nel  = string(rune(0x85))
	repl = string(rune(0xfffd))
)

func TestText(t *testing.T) {
	cases := map[string]string{
		"plain":                   "plain",
		"a\nb\tc":                 "a\nb\tc",
		"\x1b]52;c;cHduZWQ=\x07x": `\x1b]52;c;cHduZWQ=\x07x`,
		"\x1b[2J":                 `\x1b[2J`,
		"a\rb":                    `a\rb`,
		"del\x7f":                 `del\x7f`,
		"c1" + csi + "31m":        `c1\x9b31m`,
		"bad\xffbyte":             `bad\xffbyte`,
		"ok " + repl:              "ok " + repl,
		"a\u202eb":                `a\u202eb`,
		"x\u2066y\u2069":          `x\u2066y\u2069`,
		"z\u200bw\ufeff":          `z\u200bw\ufeff`,
		"p\u2028q\u2029":          `p\u2028q\u2029`,
		"soft\u00ad":              `soft\xad`,
	}
	for in, want := range cases {
		if got := Text(in); got != want {
			t.Errorf("Text(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestLine(t *testing.T) {
	if got, want := Line("a\nb\tc\x1b"), `a\nb\tc\x1b`; got != want {
		t.Fatalf("Line = %q, want %q", got, want)
	}
}

func TestHasControl(t *testing.T) {
	for _, s := range []string{"a\x1b", "a\n", nel, "a\xff", "\x7f", "a\u202e", "\u2067", "\u200b", "\u2028", "\ufeff"} {
		if !HasControl(s) {
			t.Errorf("HasControl(%q) = false", s)
		}
	}
	for _, s := range []string{"", "work/aws", "caf\xc3\xa9 " + repl} {
		if HasControl(s) {
			t.Errorf("HasControl(%q) = true", s)
		}
	}
}

func FuzzText(f *testing.F) {
	f.Add("a\x1b]52;c;x\x07\n" + csi)
	f.Fuzz(func(t *testing.T, s string) {
		for _, out := range []string{Text(s), Line(s)} {
			for _, r := range out {
				if IsControl(r) && r != '\n' && r != '\t' {
					t.Fatalf("control %q left in %q", r, out)
				}
			}
		}
		if HasControl(Line(s)) {
			t.Fatalf("Line(%q) has control", s)
		}
	})
}
