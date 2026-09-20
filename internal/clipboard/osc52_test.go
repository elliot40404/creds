package clipboard

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

func TestSequencePlain(t *testing.T) {
	got, err := Sequence("hi there", false)
	if err != nil {
		t.Fatal(err)
	}
	want := "\x1b]52;c;aGkgdGhlcmU=\x07"
	if string(got) != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestSequenceTmux(t *testing.T) {
	got, err := Sequence("hi there", true)
	if err != nil {
		t.Fatal(err)
	}
	want := "\x1bPtmux;\x1b\x1b]52;c;aGkgdGhlcmU=\x07\x1b\\"
	if string(got) != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestSequenceEmpty(t *testing.T) {
	got, err := Sequence("", false)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "\x1b]52;c;\x07" {
		t.Fatalf("got %q", got)
	}
}

func TestSequenceSizeLimit(t *testing.T) {
	if _, err := Sequence(strings.Repeat("a", MaxOSC52), false); err != nil {
		t.Fatalf("at limit: %v", err)
	}
	if _, err := Sequence(strings.Repeat("a", MaxOSC52+1), true); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("over limit: %v", err)
	}
}

func TestTerminalWrite(t *testing.T) {
	var buf bytes.Buffer
	if err := (Terminal{W: &buf, Tmux: true}).Write("x"); err != nil {
		t.Fatal(err)
	}
	if buf.String() != "\x1bPtmux;\x1b\x1b]52;c;eA==\x07\x1b\\" {
		t.Fatalf("got %q", buf.String())
	}
}

func TestTerminalWriteTooLarge(t *testing.T) {
	var buf bytes.Buffer
	err := (Terminal{W: &buf}).Write(strings.Repeat("a", MaxOSC52+1))
	if !errors.Is(err, ErrTooLarge) || buf.Len() != 0 {
		t.Fatalf("err = %v, wrote %d", err, buf.Len())
	}
}
