package render

import "strings"

func psqlInput(s string) (string, bool, error) {
	s = strings.TrimLeft(s, spaces)
	i := strings.IndexAny(s, spaces)
	if s == "psql" {
		return "", true, invalid("psql connection argument is missing")
	}
	if i < 0 || s[:i] != "psql" {
		return s, false, nil
	}
	arg := strings.Trim(s[i:], spaces)
	if arg == "" {
		return "", true, invalid("psql connection argument is missing")
	}
	if arg[0] != '\'' && arg[0] != '"' {
		if strings.ContainsAny(arg, spaces) {
			return "", true, invalid("psql needs one quoted connection string")
		}
		return arg, true, nil
	}
	value, err := psqlLiteral(arg)
	return value, true, err
}

func psqlLiteral(arg string) (string, error) {
	quote := arg[0]
	var b strings.Builder
	for i := 1; i < len(arg); i++ {
		if quote == '\'' && strings.HasPrefix(arg[i:], `'\''`) {
			b.WriteByte('\'')
			i += 3
			continue
		}
		if arg[i] == quote {
			if i == len(arg)-1 {
				return b.String(), nil
			}
			if arg[i+1] != quote {
				return "", invalid("psql needs one connection argument")
			}
			i++
		}
		b.WriteByte(arg[i])
	}
	return "", invalid("psql connection quote is not closed")
}
