package sshdir

import (
	"errors"
	"strings"
)

var (
	ErrSyntax      = errors.New("ssh config line not understood")
	ErrToken       = errors.New("ssh config tokens are not supported, line skipped")
	ErrOutside     = errors.New("path is outside the ssh dir")
	ErrIncludeLoop = errors.New("ssh config include loop")
)

const (
	keyInclude  = "include"
	keyIdentity = "identityfile"
)

type directive struct {
	key  string
	args []string
	line int
	err  error
}

func parseConfig(data []byte) []directive {
	var out []directive
	n := 0
	for line := range strings.Lines(string(data)) {
		n++
		key, rest := splitKeyword(line)
		key = strings.ToLower(key)
		if key != keyInclude && key != keyIdentity {
			continue
		}
		args, err := splitArgs(rest)
		out = append(out, directive{key: key, args: args, line: n, err: err})
	}
	return out
}

func splitKeyword(line string) (string, string) {
	line = strings.TrimSpace(line)
	i := strings.IndexAny(line, " \t=")
	if i < 0 {
		return line, ""
	}
	rest := strings.TrimLeft(line[i:], " \t")
	rest = strings.TrimPrefix(rest, "=")
	return line[:i], strings.TrimLeft(rest, " \t")
}

type argSplitter struct {
	args  []string
	cur   strings.Builder
	quote byte
	inArg bool
}

func splitArgs(s string) ([]string, error) {
	var a argSplitter
	for i := 0; i < len(s); i++ {
		if a.escaped(s, i) {
			i++
			a.add(s[i])
			continue
		}
		if a.step(s[i]) {
			break
		}
	}
	if a.quote != 0 {
		return nil, ErrSyntax
	}
	a.flush()
	return a.args, nil
}

func (a *argSplitter) escaped(s string, i int) bool {
	if s[i] != '\\' || i+1 >= len(s) {
		return false
	}
	switch c := s[i+1]; c {
	case '\\', '"', '\'':
		return true
	case ' ', '\t':
		return a.quote == 0
	}
	return false
}

func (a *argSplitter) step(c byte) bool {
	switch {
	case a.quote != 0 && c == a.quote:
		a.quote = 0
	case a.quote != 0:
		a.add(c)
	case c == '"' || c == '\'':
		a.quote = c
		a.inArg = true
	case c == ' ' || c == '\t':
		a.flush()
	case c == '#' && !a.inArg:
		return true
	default:
		a.add(c)
	}
	return false
}

func (a *argSplitter) add(c byte) {
	a.cur.WriteByte(c)
	a.inArg = true
}

func (a *argSplitter) flush() {
	if a.inArg {
		a.args = append(a.args, a.cur.String())
		a.cur.Reset()
		a.inArg = false
	}
}
