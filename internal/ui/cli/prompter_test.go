package cli

import (
	"bytes"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/elliot40404/creds/internal/app"
)

func ttyPipe(t *testing.T, input string) (*Terminal, *bytes.Buffer) {
	t.Helper()
	term, out := pipeTerminal(t, input)
	term.tty = func() bool { return true }
	return term, out
}

func pipeTerminal(t *testing.T, input string) (*Terminal, *bytes.Buffer) {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = r.Close() })
	if _, err := w.WriteString(input); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	return NewTerminal(r, &out), &out
}

func TestTerminalPasswordNeedsTTY(t *testing.T) {
	t.Parallel()
	term, _ := pipeTerminal(t, "hunter\n")
	if _, err := term.Password("Master password"); !errors.Is(err, ErrNotTerminal) {
		t.Fatalf("err = %v", err)
	}
}

func TestTerminalInput(t *testing.T) {
	t.Parallel()
	term, out := ttyPipe(t, "alice\n\nlast")
	cases := []struct{ def, want string }{{"bob", "alice"}, {"bob", "bob"}, {"", "last"}}
	for _, c := range cases {
		got, err := term.Input("User", c.def)
		if err != nil || got != c.want {
			t.Fatalf("got %q, %v want %q", got, err, c.want)
		}
	}
	if _, err := term.Input("User", "x"); !errors.Is(err, ErrNoInput) {
		t.Fatalf("eof err = %v", err)
	}
	if !strings.Contains(out.String(), "User [bob]: ") {
		t.Fatalf("prompt = %q", out.String())
	}
}

func TestTerminalConfirm(t *testing.T) {
	t.Parallel()
	term, out := ttyPipe(t, "y\nYES\nn\n\nmaybe\nno\nx\ny\nz\nq\nw\n")
	for _, want := range []bool{true, true, false, false, false, true} {
		got, err := term.Confirm("Sure")
		if err != nil || got != want {
			t.Fatalf("got %v, %v want %v", got, err, want)
		}
	}
	if !strings.Contains(out.String(), `invalid choice: "maybe", type y or n, try again`) {
		t.Fatalf("no retry note: %q", out.String())
	}
	if _, err := term.Confirm("Sure"); !errors.Is(err, ErrBadChoice) {
		t.Fatalf("err = %v", err)
	}
}

func TestTerminalConfirmRawConsole(t *testing.T) {
	t.Parallel()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = r.Close(); _ = w.Close() })
	var out bytes.Buffer
	term := NewTerminal(r, &out)
	term.tty = func() bool { return true }
	steps := []struct {
		typed string
		want  bool
	}{{"y\r", true}, {"n\r\n", false}, {"yes\r\n", true}, {"\r", false}, {"y\n", true}}
	for _, s := range steps {
		got := make(chan bool, 1)
		go func() {
			ok, err := term.Confirm("Sure")
			if err != nil {
				t.Error(err)
			}
			got <- ok
		}()
		if _, err := w.WriteString(s.typed); err != nil {
			t.Fatal(err)
		}
		select {
		case ok := <-got:
			if ok != s.want {
				t.Fatalf("typed %q: got %v want %v", s.typed, ok, s.want)
			}
		case <-time.After(5 * time.Second):
			t.Fatalf("typed %q: confirm still waiting for input", s.typed)
		}
	}
}

func TestTerminalSelect(t *testing.T) {
	t.Parallel()
	term, out := ttyPipe(t, "2\n9\nx\n1\n0\n3\nz\n")
	opts := []string{"a", "b"}
	for _, want := range []int{1, 0} {
		if n, err := term.Select("Pick", opts); err != nil || n != want {
			t.Fatalf("got %d, %v", n, err)
		}
	}
	if _, err := term.Select("Pick", opts); !errors.Is(err, ErrBadChoice) {
		t.Fatalf("err = %v", err)
	}
	if !strings.Contains(out.String(), "2) b") || !strings.Contains(out.String(), "type a number from 1 to 2, try again") {
		t.Fatalf("out %q", out.String())
	}
	if strings.Count(out.String(), "2) b") != 3 {
		t.Fatalf("options not shown once per select: %q", out.String())
	}
}

func TestTerminalShow(t *testing.T) {
	t.Parallel()
	term, out := pipeTerminal(t, "")
	if err := term.Show("Code", "ABC-DEF"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "ABC-DEF") {
		t.Fatalf("out = %q", out.String())
	}
}

func TestTerminalNeverPromptsWithoutTTY(t *testing.T) {
	t.Parallel()
	term, out := pipeTerminal(t, "y\n1\nalice\n")
	term.tty = func() bool { return false }
	if _, err := term.Confirm("Sure"); !errors.Is(err, errNeedYes) {
		t.Fatalf("confirm err = %v", err)
	}
	if _, err := term.Select("Pick", []string{"a"}); !errors.Is(err, errNeedFlags) {
		t.Fatalf("select err = %v", err)
	}
	if _, err := term.Input("User", ""); !errors.Is(err, errNeedFlags) {
		t.Fatalf("input err = %v", err)
	}
	if _, err := term.Password("pw"); !errors.Is(err, ErrNotTerminal) {
		t.Fatalf("password err = %v", err)
	}
	if out.Len() != 0 || term.Interactive() {
		t.Fatalf("prompted %q", out.String())
	}
}

func TestTerminalWarn(t *testing.T) {
	t.Parallel()
	term, out := ttyPipe(t, "")
	app.Warn(term, "bad \x1b[31minput")
	if strings.Contains(out.String(), "\x1b") || !strings.HasPrefix(out.String(), "bad ") || !strings.HasSuffix(out.String(), "input\n") {
		t.Fatalf("warn %q", out.String())
	}
}
