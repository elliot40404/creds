package search

import (
	"slices"
	"strings"
)

const (
	KeyType   = "type"
	KeyEngine = "engine"
	KeyTag    = "tag"
)

type filter struct {
	Types   []string
	Engines []string
	Tags    []string
	Text    string
}

func parseFilter(query string) filter {
	var f filter
	var text []string
	for tok := range strings.FieldsSeq(query) {
		key, val, ok := strings.Cut(tok, ":")
		if !ok || val == "" {
			text = append(text, tok)
			continue
		}
		switch strings.ToLower(key) {
		case KeyType:
			f.Types = append(f.Types, val)
		case KeyEngine:
			f.Engines = append(f.Engines, val)
		case KeyTag:
			f.Tags = append(f.Tags, val)
		default:
			text = append(text, tok)
		}
	}
	f.Text = strings.Join(text, " ")
	return f
}

func FilterText(query string) string {
	return strings.TrimSpace(parseFilter(query).Text)
}

func (f filter) match(s Summary) bool {
	if len(f.Types) > 0 && !containsFold(f.Types, string(s.Type)) {
		return false
	}
	if len(f.Engines) > 0 && !containsFold(f.Engines, s.Engine) {
		return false
	}
	if len(f.Tags) > 0 && !slices.ContainsFunc(f.Tags, func(tag string) bool { return containsFold(s.Tags, tag) }) {
		return false
	}
	return true
}

func containsFold(list []string, want string) bool {
	return slices.ContainsFunc(list, func(got string) bool { return strings.EqualFold(got, want) })
}

type Sort string

const (
	SortPath    Sort = "path"
	SortUpdated Sort = "updated"
	SortCreated Sort = "created"
)

var sorts = []Sort{SortPath, SortUpdated, SortCreated}

func Sorts() []string {
	out := make([]string, len(sorts))
	for i, s := range sorts {
		out[i] = string(s)
	}
	return out
}

func (s Sort) Valid() bool {
	return slices.Contains(sorts, s)
}

func (s Sort) Next() Sort {
	i := slices.Index(sorts, s)
	if i < 0 {
		return SortPath
	}
	return sorts[(i+1)%len(sorts)]
}

func TokenValue(query, key string) string {
	for tok := range strings.FieldsSeq(query) {
		if k, v, ok := strings.Cut(tok, ":"); ok && strings.EqualFold(k, key) {
			return v
		}
	}
	return ""
}

func WithToken(query, key, value string) string {
	var out []string
	for tok := range strings.FieldsSeq(query) {
		if k, _, ok := strings.Cut(tok, ":"); ok && strings.EqualFold(k, key) {
			continue
		}
		out = append(out, tok)
	}
	if value != "" {
		out = append([]string{key + ":" + value}, out...)
	}
	return strings.Join(out, " ")
}
