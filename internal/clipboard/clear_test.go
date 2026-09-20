package clipboard

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"
	"time"
)

const secretValue = "s3cret-clip-value"

type fakeNative struct {
	value   string
	readErr error
	clears  int
}

func (f *fakeNative) Read() (string, error) { return f.value, f.readErr }

func (f *fakeNative) Write(v string) error {
	f.value = v
	return nil
}

func (f *fakeNative) Clear() error {
	f.clears++
	f.value = ""
	return nil
}

func TestHash(t *testing.T) {
	h := Hash(secretValue)
	if len(h) != 64 || strings.Contains(h, secretValue) || h == Hash("other") {
		t.Fatalf("hash = %q", h)
	}
}

func TestReadHash(t *testing.T) {
	h := Hash(secretValue)
	for _, in := range []string{h, h + "\n", h + "\r\n"} {
		got, err := readHash(strings.NewReader(in))
		if err != nil || got != h {
			t.Fatalf("%q: %q %v", in, got, err)
		}
	}
	for _, in := range []string{"", "zz", h[:62], h + "00", secretValue} {
		if _, err := readHash(strings.NewReader(in)); !errors.Is(err, ErrBadHash) {
			t.Fatalf("%q: err = %v", in, err)
		}
	}
}

func TestParseMode(t *testing.T) {
	for _, m := range []Mode{ModeAuto, ModeNative, ModeOSC52} {
		got, err := ParseMode(m.String())
		if err != nil || got != m {
			t.Fatalf("%v: %v %v", m, got, err)
		}
	}
	if _, err := ParseMode("x11"); !errors.Is(err, ErrBadMode) {
		t.Fatalf("err = %v", err)
	}
}

func TestClearIfUnchanged(t *testing.T) {
	f := &fakeNative{value: secretValue}
	ok, err := clearIfUnchanged(f, Hash(secretValue))
	if err != nil || !ok || f.value != "" || f.clears != 1 {
		t.Fatalf("ok %v err %v %+v", ok, err, f)
	}
}

func TestClearSkipsChangedClipboard(t *testing.T) {
	f := &fakeNative{value: "user copied this"}
	ok, err := clearIfUnchanged(f, Hash(secretValue))
	if err != nil || ok || f.clears != 0 || f.value != "user copied this" {
		t.Fatalf("ok %v err %v %+v", ok, err, f)
	}
}

func TestClearSkipsOnReadError(t *testing.T) {
	f := &fakeNative{value: secretValue, readErr: io.ErrUnexpectedEOF}
	if _, err := clearIfUnchanged(f, Hash(secretValue)); err == nil || f.clears != 0 {
		t.Fatalf("err %v clears %d", err, f.clears)
	}
}

func TestClearRunNative(t *testing.T) {
	f := &fakeNative{value: secretValue}
	var slept time.Duration
	run := ClearRun{
		In:     strings.NewReader(Hash(secretValue) + "\n"),
		After:  45 * time.Second,
		Mode:   ModeNative,
		Native: f,
		Sleep:  func(d time.Duration) { slept = d },
	}
	if err := run.Run(); err != nil {
		t.Fatal(err)
	}
	if slept != 45*time.Second || f.clears != 1 {
		t.Fatalf("slept %v clears %d", slept, f.clears)
	}
}

func TestClearRunOSC52(t *testing.T) {
	var buf bytes.Buffer
	run := ClearRun{
		In:       strings.NewReader(Hash(secretValue)),
		Mode:     ModeOSC52,
		Terminal: Terminal{W: &buf, Tmux: true},
		Sleep:    func(time.Duration) {},
	}
	if err := run.Run(); err != nil {
		t.Fatal(err)
	}
	if buf.String() != "\x1bPtmux;\x1b\x1b]52;c;\x07\x1b\\" {
		t.Fatalf("got %q", buf.String())
	}
}

func TestClearRunBadInput(t *testing.T) {
	slept := false
	run := ClearRun{
		In:     strings.NewReader(secretValue),
		Mode:   ModeNative,
		Native: &fakeNative{},
		Sleep:  func(time.Duration) { slept = true },
	}
	if err := run.Run(); !errors.Is(err, ErrBadHash) || slept {
		t.Fatalf("err %v slept %v", err, slept)
	}
	run.In = strings.NewReader(Hash(secretValue))
	run.Mode = ModeAuto
	if err := run.Run(); !errors.Is(err, ErrBadMode) {
		t.Fatalf("err = %v", err)
	}
}
