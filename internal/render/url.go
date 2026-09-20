package render

import (
	"net"
	"net/url"
	"slices"
	"strings"
)

func parseURL(s string, schemes ...string) (Conn, error) {
	scheme, rest, ok := strings.Cut(s, "://")
	if !ok || !slices.Contains(schemes, scheme) {
		return Conn{}, invalid("scheme")
	}
	if strings.Contains(rest, "#") {
		return Conn{}, invalid("fragment")
	}
	c := Conn{Scheme: scheme}
	rest, query, _ := strings.Cut(rest, "?")
	authority, path, _ := strings.Cut(rest, "/")
	if i := strings.LastIndexByte(authority, '@'); i >= 0 {
		if err := c.setUserinfo(authority[:i]); err != nil {
			return Conn{}, err
		}
		authority = authority[i+1:]
	}
	if err := c.setHosts(authority); err != nil {
		return Conn{}, err
	}
	db, err := url.PathUnescape(path)
	if err != nil {
		return Conn{}, invalid("database")
	}
	c.Database = db
	c.Params, err = parseQuery(query)
	return c, err
}

func (c *Conn) setUserinfo(s string) error {
	user, pass, _ := strings.Cut(s, ":")
	var err1, err2 error
	c.Username, err1 = url.PathUnescape(user)
	c.Password, err2 = url.PathUnescape(pass)
	if err1 != nil || err2 != nil {
		return invalid("userinfo")
	}
	return nil
}

func parseQuery(q string) (map[string]string, error) {
	vals, err := url.ParseQuery(q)
	if err != nil {
		return nil, invalid("params")
	}
	params := make(map[string]string, len(vals))
	for k, v := range vals {
		params[k] = v[0]
	}
	return params, nil
}

func buildURL(scheme string, c Conn) string {
	u := url.URL{Scheme: scheme, Host: hostPort(c.Host, c.Port)}
	switch {
	case c.Password != "":
		u.User = url.UserPassword(c.Username, c.Password)
	case c.Username != "":
		u.User = url.User(c.Username)
	}
	if c.Database != "" {
		u.Path = "/" + c.Database
	} else if len(c.Params) > 0 || c.Host == "" {
		u.Path = "/"
	}
	q := make(url.Values, len(c.Params))
	for k, v := range c.Params {
		q.Set(k, v)
	}
	u.RawQuery = q.Encode()
	return u.String()
}

func hostPort(host, port string) string {
	switch {
	case strings.Contains(host, ","):
		return host
	case port != "":
		return net.JoinHostPort(host, port)
	case strings.Contains(host, ":"):
		return "[" + host + "]"
	}
	return host
}
