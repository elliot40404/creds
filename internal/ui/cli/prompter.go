package cli

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strconv"
	"strings"

	"github.com/elliot40404/creds/internal/app"
	"github.com/elliot40404/creds/internal/safetext"
	"golang.org/x/term"
)

var (
	ErrNotTerminal = errors.New("secret input needs an interactive terminal")
	ErrNoInput     = errors.New("no input")
	ErrBadChoice   = errors.New("invalid choice")

	errNeedYes   = errors.New("confirmation needs an interactive terminal")
	errNeedFlags = errors.New("input needs an interactive terminal")
)

const exitInterrupted = 130

type Terminal struct {
	in     *os.File
	out    io.Writer
	r      *bufio.Reader
	tty    func() bool
	skipLF bool
}

func NewTerminal(in *os.File, out io.Writer) *Terminal {
	t := &Terminal{in: in, out: out, r: bufio.NewReader(in)}
	t.tty = func() bool { return term.IsTerminal(int(t.in.Fd())) }
	return t
}

func (t *Terminal) Password(prompt string) (string, error) {
	if !t.tty() {
		return "", ErrNotTerminal
	}
	t.say("%s: ", prompt)
	defer t.restoreOnInterrupt()()
	b, err := term.ReadPassword(int(t.in.Fd()))
	t.say("\n")
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func (t *Terminal) Confirm(prompt string) (bool, error) {
	if !t.tty() {
		return false, errNeedYes
	}
	return app.Retry(t, func() (bool, error) {
		ans, err := t.line(prompt + " [y/N]: ")
		if err != nil {
			return false, err
		}
		switch strings.ToLower(ans) {
		case "y", "yes":
			return true, nil
		case "", "n", "no":
			return false, nil
		}
		return false, fmt.Errorf("%w: %q, type y or n", ErrBadChoice, ans)
	}, ErrBadChoice)
}

func (t *Terminal) Input(prompt, def string) (string, error) {
	p := prompt + ": "
	if def != "" {
		p = fmt.Sprintf("%s [%s]: ", prompt, def)
	}
	ans, err := t.line(p)
	if err != nil {
		return "", err
	}
	if ans == "" {
		return def, nil
	}
	return ans, nil
}

func (t *Terminal) Select(prompt string, options []string) (int, error) {
	if !t.tty() {
		return 0, errNeedFlags
	}
	for i, o := range options {
		t.say("  %d) %s\n", i+1, o)
	}
	return app.Retry(t, func() (int, error) {
		ans, err := t.line(prompt + ": ")
		if err != nil {
			return 0, err
		}
		n, err := strconv.Atoi(ans)
		if err != nil || n < 1 || n > len(options) {
			return 0, fmt.Errorf("%w: %q, type a number from 1 to %d", ErrBadChoice, ans, len(options))
		}
		return n - 1, nil
	}, ErrBadChoice)
}

func (t *Terminal) Warn(msg string) {
	t.say("%s\n", msg)
}

func (t *Terminal) Interactive() bool {
	return t.tty()
}

func (t *Terminal) Show(title, text string) error {
	_, err := fmt.Fprint(t.out, safetext.Text(fmt.Sprintf("%s:\n\n  %s\n\n", title, text)))
	return err
}

func (t *Terminal) line(prompt string) (string, error) {
	if !t.tty() {
		return "", errNeedFlags
	}
	cookInput(t.in)
	t.say("%s", prompt)
	s, err := t.readLine()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(s), nil
}

func (t *Terminal) readLine() (string, error) {
	var b strings.Builder
	for {
		c, err := t.r.ReadByte()
		if errors.Is(err, io.EOF) && b.Len() > 0 {
			return b.String(), nil
		}
		if errors.Is(err, io.EOF) {
			return "", ErrNoInput
		}
		if err != nil {
			return "", err
		}
		afterCR := t.skipLF
		t.skipLF = c == '\r'
		switch {
		case c == '\n' && afterCR:
		case c == '\n' || c == '\r':
			return b.String(), nil
		default:
			b.WriteByte(c)
		}
	}
}

func (t *Terminal) restoreOnInterrupt() func() {
	fd := int(t.in.Fd())
	st, err := term.GetState(fd)
	if err != nil {
		return func() {}
	}
	sig := make(chan os.Signal, 1)
	done := make(chan struct{})
	signal.Notify(sig, os.Interrupt)
	go func() {
		select {
		case <-sig:
			_ = term.Restore(fd, st)
			t.say("\n")
			os.Exit(exitInterrupted)
		case <-done:
		}
	}()
	return func() {
		signal.Stop(sig)
		close(done)
	}
}

func (t *Terminal) say(format string, a ...any) {
	_, _ = fmt.Fprint(t.out, safetext.Text(fmt.Sprintf(format, a...)))
}
