package config

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestTrimNameKeepsWholeRunes(t *testing.T) {
	got := trimName(strings.Repeat("a", maxMachineName-1) + "é")
	if !utf8.ValidString(got) || len(got) > maxMachineName {
		t.Fatalf("got %q (%d bytes)", got, len(got))
	}
	if short := trimName("box"); short != "box" {
		t.Fatalf("got %q", short)
	}
}
