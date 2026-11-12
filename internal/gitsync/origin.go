package gitsync

import (
	"context"
	"errors"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/elliot40404/creds/internal/fsutil"
)

const maxConfigSize = 64 << 10

type originState int

const (
	originUnsure originState = iota
	originNone
	originFound
)

var bannedSections = []string{"include", "includeif", "extensions"}

type originParser struct {
	inSection bool
	inOrigin  bool
	sections  int
	urls      []string
}

func HasRemote(ctx context.Context, dir string) bool {
	_, st := readOrigin(dir)
	switch st {
	case originFound:
		return true
	case originNone:
		return false
	}
	url, err := Open(dir).gitRemoteURL(ctx)
	return err == nil && url != ""
}

func readOrigin(dir string) (string, originState) {
	data, err := fsutil.ReadFileLimit(filepath.Join(dir, ".git", "config"), maxConfigSize)
	if errors.Is(err, fs.ErrNotExist) {
		return "", originNone
	}
	if err != nil {
		return "", originUnsure
	}
	return parseOrigin(string(data))
}

func parseOrigin(data string) (string, originState) {
	if strings.ContainsAny(data, "\r\x00") || strings.HasPrefix(data, "\xef\xbb\xbf") {
		return "", originUnsure
	}
	var p originParser
	for line := range strings.SplitSeq(data, "\n") {
		if !p.line(strings.TrimLeft(line, " \t")) {
			return "", originUnsure
		}
	}
	return p.result()
}

func (p *originParser) line(t string) bool {
	switch {
	case t == "" || t[0] == '#' || t[0] == ';':
		return true
	case t[0] == '[':
		return p.header(t)
	}
	return p.key(t)
}

func (p *originParser) header(t string) bool {
	end := strings.IndexByte(t, ']')
	if end < 0 || !isComment(strings.TrimSpace(t[end+1:])) {
		return false
	}
	name, sub, hasSub := strings.Cut(t[1:end], " ")
	if !validName(name) || isBanned(name) {
		return false
	}
	if hasSub {
		var ok bool
		if sub, ok = subsection(sub); !ok {
			return false
		}
	}
	p.inSection = true
	p.inOrigin = strings.EqualFold(name, "remote") && hasSub && sub == Remote
	if p.inOrigin {
		p.sections++
	}
	return true
}

func (p *originParser) key(t string) bool {
	name, val, hasEq := strings.Cut(t, "=")
	if !p.inSection || !validName(strings.TrimRight(name, " \t")) || !startsWithLetter(name) {
		return false
	}
	if p.inOrigin && strings.EqualFold(strings.TrimRight(name, " \t"), "url") {
		if !hasEq {
			return false
		}
		p.urls = append(p.urls, val)
		return true
	}
	return !strings.HasSuffix(strings.TrimRight(val, " \t"), `\`) && strings.Count(val, `"`)%2 == 0
}

func (p *originParser) result() (string, originState) {
	if p.sections == 0 {
		return "", originNone
	}
	if p.sections != 1 || len(p.urls) != 1 {
		return "", originUnsure
	}
	url, ok := plainValue(p.urls[0])
	if !ok {
		return "", originUnsure
	}
	return url, originFound
}

func plainValue(raw string) (string, bool) {
	v := strings.Trim(raw, " \t")
	if v == "" || strings.ContainsAny(v, " \t\"#;") || hasControl(v) {
		return "", false
	}
	var b strings.Builder
	for i := 0; i < len(v); i++ {
		if v[i] != '\\' {
			b.WriteByte(v[i])
			continue
		}
		if i+1 >= len(v) || v[i+1] != '\\' {
			return "", false
		}
		b.WriteByte('\\')
		i++
	}
	return b.String(), true
}

func subsection(s string) (string, bool) {
	if len(s) < 2 || s[0] != '"' || s[len(s)-1] != '"' {
		return "", false
	}
	inner := s[1 : len(s)-1]
	return inner, !strings.ContainsAny(inner, "\"\\")
}

func validName(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if !isAlnum(c) && c != '-' {
			return false
		}
	}
	return true
}

func startsWithLetter(s string) bool {
	c := s[0]
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

func isAlnum(c rune) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
}

func isComment(s string) bool {
	return s == "" || s[0] == '#' || s[0] == ';'
}

func isBanned(name string) bool {
	for _, b := range bannedSections {
		if strings.EqualFold(name, b) {
			return true
		}
	}
	return false
}

func hasControl(s string) bool {
	return strings.ContainsFunc(s, func(r rune) bool { return r < 0x20 || r == 0x7f })
}
