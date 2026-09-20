package safetext

import (
	"net/url"
	"slices"
	"strings"
)

var secretQuery = []string{"access_token", "token", "private_token", "oauth_token"}

func Remote(raw string) string {
	u, err := url.Parse(raw)
	if err == nil {
		query, redacted := redactQuery(u.RawQuery)
		if u.User == nil && !redacted {
			return raw
		}
		u.User, u.RawQuery = nil, query
		return u.String()
	}
	scheme, rest, ok := strings.Cut(raw, "://")
	if !ok {
		return raw
	}
	rest, _, _ = strings.Cut(rest, "?")
	end := strings.IndexByte(rest, '/')
	if end < 0 {
		end = len(rest)
	}
	if i := strings.LastIndexByte(rest[:end], '@'); i >= 0 {
		rest = rest[i+1:]
	}
	return scheme + "://" + rest
}

func redactQuery(raw string) (string, bool) {
	if raw == "" {
		return raw, false
	}
	parts := strings.Split(raw, "&")
	redacted := false
	for i, p := range parts {
		k, _, ok := strings.Cut(p, "=")
		if ok && slices.Contains(secretQuery, strings.ToLower(k)) {
			parts[i], redacted = k+"=redacted", true
		}
	}
	return strings.Join(parts, "&"), redacted
}
