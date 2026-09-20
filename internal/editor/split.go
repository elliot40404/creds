package editor

import (
	"strings"
	"unicode"
)

func split(line string) []string {
	var out []string
	var cur strings.Builder
	var quote rune
	started := false
	for _, r := range line {
		switch {
		case quote != 0 && r == quote:
			quote = 0
		case quote == 0 && (r == '"' || r == '\''):
			quote, started = r, true
		case quote == 0 && unicode.IsSpace(r):
			if started {
				out = append(out, cur.String())
				cur.Reset()
				started = false
			}
		default:
			cur.WriteRune(r)
			started = true
		}
	}
	if started {
		out = append(out, cur.String())
	}
	return out
}
