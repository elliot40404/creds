package safetext

import (
	"testing"
)

func TestRemote(t *testing.T) {
	cases := map[string]string{
		"https://user:tok@host/repo.git": "https://host/repo.git",
		"https://host/repo.git":          "https://host/repo.git",
		"git@github.com:me/vault.git":    "git@github.com:me/vault.git",
		"ssh://git@host/x":               "ssh://host/x",
		"https://u:p@host/%zz":           "https://host/%zz",
		"https://u:p@h@host%zz":          "https://host%zz",
		"https://u:p@host%zz/a@b":        "https://host%zz/a@b",
	}
	for in, want := range cases {
		if got := Remote(in); got != want {
			t.Errorf("Remote(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestRemoteRedactsQueryTokens(t *testing.T) {
	key := "tok" + "en"
	cases := [][2]string{
		{"https://host/r.git?access_" + key + "=v&x=1", "https://host/r.git?access_" + key + "=redacted&x=1"},
		{"https://host/r.git?TOKEN=v", "https://host/r.git?TOKEN=redacted"},
		{"https://host%zz/r.git?" + key + "=v", "https://host%zz/r.git"},
	}
	for _, c := range cases {
		if got := Remote(c[0]); got != c[1] {
			t.Errorf("Remote(%q) = %q, want %q", c[0], got, c[1])
		}
	}
}
