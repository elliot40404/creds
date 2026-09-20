package editor

import (
	"slices"
	"testing"
)

func TestCommandKeepsQuotedPaths(t *testing.T) {
	cases := map[string][]string{
		`"C:\Program Files\Microsoft VS Code\bin\code.cmd" --wait`: {`C:\Program Files\Microsoft VS Code\bin\code.cmd`, "--wait"},
		"vim -f":               {"vim", "-f"},
		`'my editor' -x "a b"`: {"my editor", "-x", "a b"},
		"  nano  ":             {"nano"},
	}
	for line, want := range cases {
		t.Setenv("VISUAL", line)
		name, args, err := command()
		if err != nil || !slices.Equal(append([]string{name}, args...), want) {
			t.Errorf("%q: got %q %q %v", line, name, args, err)
		}
	}
}
