package safetext

import (
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

func Text(s string) string {
	return clean(s, false)
}

func Line(s string) string {
	return clean(s, true)
}

func HasControl(s string) bool {
	if !utf8.ValidString(s) {
		return true
	}
	for _, r := range s {
		if IsControl(r) {
			return true
		}
	}
	return false
}

func clean(s string, line bool) string {
	if !dirty(s, line) {
		return s
	}
	var b strings.Builder
	b.Grow(len(s) + 16)
	for i := 0; i < len(s); {
		r, n := utf8.DecodeRuneInString(s[i:])
		switch {
		case r == utf8.RuneError && n == 1:
			hex(&b, int64(s[i]))
		case kept(r, line) || !IsControl(r):
			b.WriteString(s[i : i+n])
		default:
			escape(&b, r)
		}
		i += n
	}
	return b.String()
}

func dirty(s string, line bool) bool {
	for _, r := range s {
		if (IsControl(r) && !kept(r, line)) || r == utf8.RuneError {
			return true
		}
	}
	return false
}

func kept(r rune, line bool) bool {
	return !line && (r == '\n' || r == '\t')
}

func IsControl(r rune) bool {
	return r < 0x20 || (r >= 0x7f && r <= 0x9f) || r == 0x2028 || r == 0x2029 || unicode.Is(unicode.Cf, r)
}

func escape(b *strings.Builder, r rune) {
	switch {
	case r == '\n':
		b.WriteString(`\n`)
	case r == '\r':
		b.WriteString(`\r`)
	case r == '\t':
		b.WriteString(`\t`)
	case r > 0xff:
		b.WriteString(`\u`)
		pad(b, strconv.FormatInt(int64(r), 16), 4)
	default:
		hex(b, int64(r))
	}
}

func hex(b *strings.Builder, v int64) {
	b.WriteString(`\x`)
	pad(b, strconv.FormatInt(v, 16), 2)
}

func pad(b *strings.Builder, digits string, width int) {
	for range width - len(digits) {
		b.WriteByte('0')
	}
	b.WriteString(digits)
}
