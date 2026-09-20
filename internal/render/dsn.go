package render

import "strings"

const spaces = " \t\n\r"

func parseDSN(s string) (Conn, error) {
	c := Conn{Params: map[string]string{}}
	s = strings.TrimLeft(s, spaces)
	if s == "" {
		return Conn{}, invalid("empty")
	}
	for s != "" {
		key, rest, ok := strings.Cut(s, "=")
		key = strings.TrimRight(key, spaces)
		if !ok || key == "" || strings.ContainsAny(key, spaces) {
			return Conn{}, invalid("dsn key")
		}
		val, rest, err := dsnValue(strings.TrimLeft(rest, spaces))
		if err != nil {
			return Conn{}, err
		}
		if err := c.setDSN(key, val); err != nil {
			return Conn{}, err
		}
		s = strings.TrimLeft(rest, spaces)
	}
	return c, nil
}

func dsnValue(s string) (string, string, error) {
	quoted := strings.HasPrefix(s, "'")
	if quoted {
		s = s[1:]
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		ch := s[i]
		switch {
		case ch == '\\':
			i++
			if i == len(s) {
				return "", "", invalid("dsn escape")
			}
			b.WriteByte(s[i])
		case quoted && ch == '\'':
			rest := s[i+1:]
			if rest != "" && !strings.ContainsRune(spaces, rune(rest[0])) {
				return "", "", invalid("dsn quote")
			}
			return b.String(), rest, nil
		case !quoted && strings.ContainsRune(spaces, rune(ch)):
			return b.String(), s[i:], nil
		default:
			b.WriteByte(ch)
		}
	}
	if quoted {
		return "", "", invalid("dsn quote")
	}
	return b.String(), "", nil
}

func (c *Conn) setDSN(key, val string) error {
	switch key {
	case "host":
		c.Host = val
	case "port":
		c.Port = val
		return checkPort(val)
	case "user":
		c.Username = val
	case "password":
		c.Password = val
	case "dbname":
		c.Database = val
	default:
		c.Params[key] = val
	}
	return nil
}
