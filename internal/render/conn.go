package render

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"slices"
	"strconv"
	"strings"
)

const (
	Postgres = "postgres"
	Redis    = "redis"
	Mongo    = "mongo"
)

var (
	ErrUnknownEngine = errors.New("render: unknown engine")
	ErrInvalidConn   = errors.New("render: invalid connection string")

	secretParams = []string{"password", "sslpassword", "pass", "token", "secret"}
)

type Conn struct {
	Scheme   string
	Host     string
	Port     string
	Username string
	Password string
	Database string
	Params   map[string]string
}

func Parse(engine, s string) (Conn, error) {
	c, err := parseEngine(engine, s)
	if err != nil {
		return Conn{}, err
	}
	if p, ok := c.Params["password"]; ok {
		c.Password = p
		delete(c.Params, "password")
	}
	return c, nil
}

func SecretParam(key string) bool {
	return slices.Contains(secretParams, strings.ToLower(key))
}

func parseEngine(engine, s string) (Conn, error) {
	switch engine {
	case Postgres:
		return parsePostgres(s)
	case Redis:
		return parseRedis(s)
	case Mongo:
		return parseMongo(s)
	}
	return Conn{}, fmt.Errorf("%w: %q", ErrUnknownEngine, engine)
}

func invalid(what string) error {
	return fmt.Errorf("%w: %s", ErrInvalidConn, what)
}

func (c *Conn) setHosts(a string) error {
	if !strings.Contains(a, ",") {
		host, port, err := urlHost(a)
		c.Host, c.Port = host, port
		return err
	}
	var hosts []string
	for h := range strings.SplitSeq(a, ",") {
		host, port, err := urlHost(h)
		if err != nil || host == "" {
			return invalid("host list")
		}
		hosts = append(hosts, hostPort(host, port))
	}
	c.Host = strings.Join(hosts, ",")
	return nil
}

func urlHost(a string) (string, string, error) {
	host, port, err := splitHostPort(a)
	if err != nil {
		return "", "", err
	}
	host, err = url.PathUnescape(host)
	if err != nil || strings.ContainsAny(host, ",[]") {
		return "", "", invalid("host")
	}
	return host, port, nil
}

func splitHostPort(a string) (string, string, error) {
	if !strings.Contains(a, ":") || strings.HasPrefix(a, "[") && strings.HasSuffix(a, "]") {
		return strings.Trim(a, "[]"), "", nil
	}
	host, port, err := net.SplitHostPort(a)
	if err != nil {
		return "", "", invalid("host")
	}
	return host, port, checkPort(port)
}

func checkPort(p string) error {
	if p == "" || strings.Contains(p, ",") {
		return nil
	}
	n, err := strconv.Atoi(p)
	if err != nil || n < 1 || n > 65535 || p[0] == '+' {
		return invalid("port")
	}
	return nil
}
