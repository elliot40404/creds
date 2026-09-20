package envfile

import (
	"errors"
	"fmt"
	"io"
	"strings"
)

var (
	ErrDuplicateKey = errors.New("duplicate key")
	ErrNotDotenv    = errors.New("value cannot be written safely as dotenv")
)

func Write(w io.Writer, vars []Var) error {
	return write(w, vars, func(s string) (string, error) { return quote(s), nil })
}

func WriteDotenv(w io.Writer, vars []Var) error {
	return write(w, vars, dotenvQuote)
}

func write(w io.Writer, vars []Var, quoter func(string) (string, error)) error {
	var b strings.Builder
	seen := make(map[string]struct{}, len(vars))
	for _, v := range vars {
		if !validKey(v.Key) {
			return fmt.Errorf("%w %q", ErrBadKey, v.Key)
		}
		if _, dup := seen[v.Key]; dup {
			return fmt.Errorf("%w %s", ErrDuplicateKey, v.Key)
		}
		seen[v.Key] = struct{}{}
		if strings.IndexByte(v.Value, 0) >= 0 {
			return fmt.Errorf("%s: %w", v.Key, ErrNUL)
		}
		q, err := quoter(v.Value)
		if err != nil {
			return fmt.Errorf("%s: %w", v.Key, err)
		}
		b.WriteString(v.Key)
		b.WriteByte('=')
		b.WriteString(q)
		b.WriteByte('\n')
	}
	_, err := io.WriteString(w, b.String())
	return err
}

func quote(s string) string {
	switch {
	case isBareValue(s):
		return s
	case !strings.ContainsAny(s, "'\n\r"):
		return "'" + s + "'"
	}
	var b strings.Builder
	b.Grow(len(s) + 2)
	b.WriteByte('"')
	for i := range len(s) {
		switch c := s[i]; c {
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\\', '"', '$', '`':
			b.WriteByte('\\')
			b.WriteByte(c)
		default:
			b.WriteByte(c)
		}
	}
	b.WriteByte('"')
	return b.String()
}

func dotenvQuote(s string) (string, error) {
	switch {
	case isBareValue(s):
		return s, nil
	case !strings.ContainsAny(s, "'\n\r") && !strings.Contains(s, `\\`) && !strings.HasSuffix(s, `\`):
		return "'" + s + "'", nil
	case !strings.ContainsAny(s, "\"\\") && !strings.Contains(s, "${"):
		return `"` + strings.NewReplacer("\n", `\n`, "\r", `\r`).Replace(s) + `"`, nil
	}
	return "", ErrNotDotenv
}

func isBareValue(s string) bool {
	for i := range len(s) {
		if !isKeyChar(s[i]) && !strings.ContainsRune("./:@%+,-=", rune(s[i])) {
			return false
		}
	}
	return true
}
