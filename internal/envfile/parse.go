package envfile

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"
)

const maxSize = 1 << 20

var (
	ErrTooLarge = errors.New("env file too large")
	ErrBadKey   = errors.New("invalid key")
	ErrSyntax   = errors.New("syntax error")
	ErrNUL      = errors.New("NUL byte not allowed")
)

type Var struct {
	Key   string
	Value string
}

func Parse(r io.Reader) ([]Var, []string, error) {
	src, err := readAll(r)
	if err != nil {
		return nil, nil, err
	}
	p := parser{src: strings.ReplaceAll(src, "\r\n", "\n"), line: 1}
	return p.run()
}

func readAll(r io.Reader) (string, error) {
	data, err := io.ReadAll(io.LimitReader(r, maxSize+1))
	if err != nil {
		return "", err
	}
	if len(data) > maxSize {
		return "", ErrTooLarge
	}
	if bytes.IndexByte(data, 0) >= 0 {
		return "", ErrNUL
	}
	return string(data), nil
}

func validKey(k string) bool {
	if k == "" || isDigit(k[0]) {
		return false
	}
	for i := range len(k) {
		if !isKeyChar(k[i]) {
			return false
		}
	}
	return true
}

func isDigit(c byte) bool {
	return c >= '0' && c <= '9'
}

func isKeyChar(c byte) bool {
	return c == '_' || isDigit(c) || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

type parser struct {
	src  string
	pos  int
	line int
}

func (p *parser) run() ([]Var, []string, error) {
	var vars []Var
	var warns []string
	index := map[string]int{}
	for p.pos < len(p.src) {
		line := p.line
		v, ok, err := p.entry()
		if err != nil {
			return nil, nil, fmt.Errorf("line %d: %w", line, err)
		}
		if !ok {
			continue
		}
		if i, dup := index[v.Key]; dup {
			vars[i].Value = v.Value
			warns = append(warns, fmt.Sprintf("line %d: duplicate key %s, last value wins", line, v.Key))
			continue
		}
		index[v.Key] = len(vars)
		vars = append(vars, v)
	}
	return vars, warns, nil
}

func (p *parser) entry() (Var, bool, error) {
	p.skipSpace()
	if p.pos >= len(p.src) || p.src[p.pos] == '\n' || p.src[p.pos] == '#' {
		p.skipLine()
		return Var{}, false, nil
	}
	p.skipExport()
	key := p.key()
	if !validKey(key) {
		return Var{}, false, ErrBadKey
	}
	p.skipSpace()
	if p.pos >= len(p.src) || p.src[p.pos] != '=' {
		return Var{}, false, p.missingEquals()
	}
	p.pos++
	spaced := p.skipSpace()
	val, err := p.value(spaced)
	if err != nil {
		return Var{}, false, fmt.Errorf("%s: %w", key, err)
	}
	return Var{Key: key, Value: val}, true, nil
}

func (p *parser) missingEquals() error {
	rest := p.src[p.pos:]
	line, _, _ := strings.Cut(rest, "\n")
	if strings.Contains(line, "=") {
		return fmt.Errorf("%w: space in key", ErrBadKey)
	}
	return fmt.Errorf("%w: missing =", ErrSyntax)
}

func (p *parser) skipSpace() bool {
	start := p.pos
	for p.pos < len(p.src) && isBlank(p.src[p.pos]) {
		p.pos++
	}
	return p.pos > start
}

func (p *parser) skipLine() {
	i := strings.IndexByte(p.src[p.pos:], '\n')
	if i < 0 {
		p.pos = len(p.src)
	} else {
		p.pos += i + 1
	}
	p.line++
}

func (p *parser) finishLine() error {
	p.skipSpace()
	if p.pos < len(p.src) && p.src[p.pos] != '\n' && p.src[p.pos] != '#' {
		return fmt.Errorf("%w: unexpected text after value", ErrSyntax)
	}
	p.skipLine()
	return nil
}

func (p *parser) skipExport() {
	rest := p.src[p.pos:]
	if len(rest) > 6 && strings.HasPrefix(rest, "export") && isBlank(rest[6]) {
		p.pos += 6
		p.skipSpace()
	}
}

func (p *parser) key() string {
	start := p.pos
	for p.pos < len(p.src) && !strings.ContainsRune("= \t\n", rune(p.src[p.pos])) {
		p.pos++
	}
	return p.src[start:p.pos]
}

func (p *parser) value(spaced bool) (string, error) {
	if p.pos >= len(p.src) {
		return "", nil
	}
	switch p.src[p.pos] {
	case '\'':
		return p.single()
	case '"':
		return p.double()
	}
	return p.bare(spaced), nil
}

func (p *parser) bare(spaced bool) string {
	start := p.pos
	for p.pos < len(p.src) && p.src[p.pos] != '\n' {
		c := p.src[p.pos]
		if c == '#' && ((p.pos == start && spaced) || (p.pos > start && isBlank(p.src[p.pos-1]))) {
			break
		}
		p.pos++
	}
	val := strings.TrimRight(p.src[start:p.pos], " \t")
	p.skipLine()
	return val
}

func isBlank(c byte) bool {
	return c == ' ' || c == '\t'
}

func (p *parser) single() (string, error) {
	p.pos++
	end := strings.IndexByte(p.src[p.pos:], '\'')
	if end < 0 {
		return "", fmt.Errorf("%w: unterminated single quote", ErrSyntax)
	}
	val := p.src[p.pos : p.pos+end]
	p.line += strings.Count(val, "\n")
	p.pos += end + 1
	return val, p.finishLine()
}

func (p *parser) double() (string, error) {
	p.pos++
	var b strings.Builder
	for p.pos < len(p.src) {
		c := p.src[p.pos]
		switch c {
		case '"':
			p.pos++
			return b.String(), p.finishLine()
		case '\\':
			if p.pos+1 >= len(p.src) {
				return "", fmt.Errorf("%w: unterminated double quote", ErrSyntax)
			}
			p.unescape(&b, p.src[p.pos+1])
			p.pos += 2
			continue
		case '\n':
			p.line++
		}
		b.WriteByte(c)
		p.pos++
	}
	return "", fmt.Errorf("%w: unterminated double quote", ErrSyntax)
}

func (p *parser) unescape(b *strings.Builder, c byte) {
	switch c {
	case 'n':
		b.WriteByte('\n')
	case 'r':
		b.WriteByte('\r')
	case 't':
		b.WriteByte('\t')
	case '\\', '"', '$', '`':
		b.WriteByte(c)
	default:
		if c == '\n' {
			p.line++
		}
		b.WriteByte('\\')
		b.WriteByte(c)
	}
}
