package sshdir

import (
	"errors"
	"slices"
	"testing"
)

func TestSplitKeyword(t *testing.T) {
	tests := []struct {
		line, key, rest string
	}{
		{"IdentityFile ~/.ssh/id", "IdentityFile", "~/.ssh/id"},
		{"  identityfile=id\r\n", "identityfile", "id"},
		{"IdentityFile = id", "IdentityFile", "id"},
		{"Include\ta b", "Include", "a b"},
		{"Include", "Include", ""},
		{"", "", ""},
		{"# IdentityFile id", "#", "IdentityFile id"},
	}
	for _, tt := range tests {
		key, rest := splitKeyword(tt.line)
		if key != tt.key || rest != tt.rest {
			t.Errorf("splitKeyword(%q) = %q %q, want %q %q", tt.line, key, rest, tt.key, tt.rest)
		}
	}
}

func TestSplitArgs(t *testing.T) {
	tests := []struct {
		in   string
		want []string
		err  error
	}{
		{"a b\tc", []string{"a", "b", "c"}, nil},
		{`"with space" 'single q'`, []string{"with space", "single q"}, nil},
		{`a\ b`, []string{"a b"}, nil},
		{`C:\Users\me\.ssh\id`, []string{`C:\Users\me\.ssh\id`}, nil},
		{`"a \"q\" b"`, []string{`a "q" b`}, nil},
		{`a\\b`, []string{`a\b`}, nil},
		{"a # note", []string{"a"}, nil},
		{"a#b", []string{"a#b"}, nil},
		{`"#x"`, []string{"#x"}, nil},
		{`""`, []string{""}, nil},
		{"", nil, nil},
		{`"open`, nil, ErrSyntax},
		{`a 'open`, nil, ErrSyntax},
		{`\`, []string{`\`}, nil},
	}
	for _, tt := range tests {
		got, err := splitArgs(tt.in)
		if !errors.Is(err, tt.err) || !slices.Equal(got, tt.want) {
			t.Errorf("splitArgs(%q) = %q %v, want %q %v", tt.in, got, err, tt.want, tt.err)
		}
	}
}

func TestParseConfigKeepsOnlyIncludeAndIdentity(t *testing.T) {
	src := "Host a\n  HostName a.example\n  IdentityFile id\n# Include x\nINCLUDE x y\nUser me\n"
	got := parseConfig([]byte(src))
	want := []directive{
		{key: keyIdentity, args: []string{"id"}, line: 3},
		{key: keyInclude, args: []string{"x", "y"}, line: 5},
	}
	if len(got) != len(want) {
		t.Fatalf("got %d directives, want %d: %+v", len(got), len(want), got)
	}
	for i := range want {
		if got[i].key != want[i].key || got[i].line != want[i].line || !slices.Equal(got[i].args, want[i].args) {
			t.Errorf("directive %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}
