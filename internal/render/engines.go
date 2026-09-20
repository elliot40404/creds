package render

import (
	"slices"
	"strconv"
	"strings"
)

var engineSchemes = map[string][]string{
	Postgres: {"postgres", "postgresql"},
	Redis:    {"redis", "rediss"},
	Mongo:    {"mongodb", "mongodb+srv"},
}

func EngineFor(s string) (string, bool) {
	scheme, _, ok := strings.Cut(strings.TrimLeft(s, spaces), "://")
	if ok {
		for engine, schemes := range engineSchemes {
			if slices.Contains(schemes, scheme) {
				return engine, true
			}
		}
		return "", false
	}
	c, err := parseDSN(s)
	if err != nil {
		return "", false
	}
	if c.Host != "" || c.Database != "" || c.Username != "" || c.Port != "" {
		return Postgres, true
	}
	return "", false
}

func parsePostgres(s string) (Conn, error) {
	if looksLikeURL(s) {
		return parseURL(s, engineSchemes[Postgres]...)
	}
	return parseDSN(s)
}

func looksLikeURL(s string) bool {
	s = strings.TrimLeft(s, spaces)
	if i := strings.IndexAny(s, spaces); i >= 0 {
		s = s[:i]
	}
	return strings.Contains(s, "://")
}

func parseRedis(s string) (Conn, error) {
	c, err := parseURL(s, engineSchemes[Redis]...)
	if err != nil {
		return Conn{}, err
	}
	if c.Host == "" || strings.Contains(c.Host, ",") {
		return Conn{}, invalid("host")
	}
	if c.Database != "" {
		if n, err := strconv.Atoi(c.Database); err != nil || n < 0 || c.Database[0] == '+' {
			return Conn{}, invalid("database")
		}
	}
	return c, nil
}

func parseMongo(s string) (Conn, error) {
	c, err := parseURL(s, engineSchemes[Mongo]...)
	if err != nil {
		return Conn{}, err
	}
	if c.Host == "" {
		return Conn{}, invalid("host")
	}
	if c.Scheme == "mongodb+srv" && (c.Port != "" || strings.Contains(c.Host, ",")) {
		return Conn{}, invalid("srv host")
	}
	return c, nil
}
