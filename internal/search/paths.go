package search

import (
	"slices"
	"strings"
	"unicode/utf8"
)

func Paths(items []Summary) []string {
	out := make([]string, len(items))
	for i, it := range items {
		out[i] = it.Path
	}
	return out
}

func Prefixes(paths []string) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, rest := range paths {
		at := 0
		for {
			i := strings.IndexByte(rest[at:], '/')
			if i < 0 {
				break
			}
			at += i + 1
			folder := rest[:at]
			if _, ok := seen[folder]; !ok {
				seen[folder] = struct{}{}
				out = append(out, folder)
			}
		}
	}
	slices.Sort(out)
	return out
}

func Candidates(paths []string) []string {
	out := Prefixes(paths)
	out = append(out, paths...)
	slices.Sort(out)
	return slices.Compact(out)
}

func Complete(cands []string, typed string) (string, []string) {
	if typed == "" {
		return "", nil
	}
	var matches []string
	for _, c := range cands {
		if c != typed && strings.HasPrefix(c, typed) {
			matches = append(matches, c)
		}
	}
	if len(matches) == 0 {
		return "", nil
	}
	filled := matches[0]
	for _, m := range matches[1:] {
		filled = commonPrefix(filled, m)
	}
	return filled, matches
}

func commonPrefix(a, b string) string {
	n := min(len(a), len(b))
	i := 0
	for i < n && a[i] == b[i] {
		i++
	}
	for i > 0 && i < len(a) && !utf8.RuneStart(a[i]) {
		i--
	}
	return a[:i]
}
