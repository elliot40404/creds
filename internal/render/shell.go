package render

import (
	"errors"
	"fmt"
	"runtime"
	"strings"
	"text/template"
)

type Shell string

const (
	Bash Shell = "bash"
	Pwsh Shell = "pwsh"
)

var ErrUnknownShell = errors.New("render: unknown shell")

func DefaultShell() Shell {
	if runtime.GOOS == "windows" {
		return Pwsh
	}
	return Bash
}

func (sh Shell) Other() Shell {
	if sh == Pwsh {
		return Bash
	}
	return Pwsh
}

func Shells() []string {
	return []string{string(Bash), string(Pwsh)}
}

func ParseShell(s string) (Shell, error) {
	switch Shell(s) {
	case "":
		return DefaultShell(), nil
	case Bash, Pwsh:
		return Shell(s), nil
	}
	return "", fmt.Errorf("%w: %q (use bash or pwsh)", ErrUnknownShell, s)
}

func (sh Shell) funcs() template.FuncMap {
	pwsh := sh == Pwsh
	quote, setEnd := shellQuote, " "
	if pwsh {
		quote, setEnd = pwshQuote, "; "
	}
	assign := func(name, v, end string) string {
		switch {
		case v == "":
			return ""
		case pwsh:
			return "$env:" + name + "=" + quote(v) + end
		}
		return name + "=" + quote(v) + end
	}
	return template.FuncMap{
		"sh":      quote,
		"envset":  func(name, v string) string { return assign(name, v, setEnd) },
		"envline": func(name, v string) string { return assign(name, v, "\n") },
		"dotenv": func(name, v string) string {
			if v == "" {
				return ""
			}
			return name + "=" + dotenvQuote(v) + "\n"
		},
		"envunset": func(name, v string) string {
			if v == "" || !pwsh {
				return ""
			}
			return "; Remove-Item Env:" + name
		},
		"flag": func(prefix, v string) string {
			if pwsh {
				return quote(prefix + v)
			}
			return prefix + quote(v)
		},
	}
}

func dotenvQuote(s string) string {
	if strings.IndexFunc(s, unsafeShell) < 0 {
		return s
	}
	var b strings.Builder
	b.Grow(len(s) + 2)
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '\\', '"', '$', '`':
			b.WriteByte('\\')
		case '\n':
			b.WriteString(`\n`)
			continue
		}
		b.WriteRune(r)
	}
	b.WriteByte('"')
	return b.String()
}

func pwshQuote(s string) string {
	var b strings.Builder
	b.Grow(len(s) + 2)
	b.WriteByte('\'')
	for _, r := range s {
		if isPwshSingleQuote(r) {
			b.WriteRune(r)
		}
		b.WriteRune(r)
	}
	b.WriteByte('\'')
	return b.String()
}

func isPwshSingleQuote(r rune) bool {
	switch r {
	case '\'', '‘', '’', '‚', '‛':
		return true
	}
	return false
}

func shellQuote(s string) string {
	if s != "" && strings.IndexFunc(s, unsafeShell) < 0 {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func unsafeShell(r rune) bool {
	switch {
	case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		return false
	}
	return !strings.ContainsRune("@%+=:,./-_", r)
}

func (sh Shell) Quote(s string) string {
	if sh != Pwsh {
		return shellQuote(s)
	}
	if s != "" && strings.IndexFunc(s, unsafePwsh) < 0 {
		return s
	}
	return pwshQuote(s)
}

func unsafePwsh(r rune) bool {
	return r == ',' || r == '@' || unsafeShell(r)
}
