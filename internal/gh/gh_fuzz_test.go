package gh

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func FuzzCheckRepoName(f *testing.F) {
	for _, s := range []string{
		"", "creds-vault", "-rf", "--private", "a/b", "a b", "a\nb", "a\x00b", "..", "ünï", strings.Repeat("a", 200),
		"ext::sh -c id", "file:///tmp", "a.b_c-1",
	} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, name string) {
		if err := checkRepoName(name); err != nil {
			return
		}
		if name == "" || strings.HasPrefix(name, "-") || len(name) > 100 {
			t.Fatalf("accepted bad name %q", name)
		}
		for _, r := range name {
			ok := r == '-' || r == '_' || r == '.' || r >= '0' && r <= '9' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z'
			if !ok {
				t.Fatalf("accepted name %q with rune %q", name, r)
			}
		}
	})
}

func FuzzRepos(f *testing.F) {
	for _, s := range []string{
		"", "\n", "  \n  ", "git@github.com:a/b.git\ngit@github.com:c/d.git\n",
		"git@github.com:a/b.git\r\n\r\nx", "\x00\x1b]52;c;aaa\x07", strings.Repeat("u\n", 300),
		"gh: could not find any repositories", "ext::sh -c id", "-u ssh://x",
	} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, out string) {
		g := &Client{Run: func(_ context.Context, _ ...string) ([]byte, error) { return []byte(out), nil }}
		urls, err := g.Repos(context.Background())
		if err != nil {
			t.Fatalf("repos: %v", err)
		}
		for _, u := range urls {
			if u == "" || u != strings.TrimSpace(u) {
				t.Fatalf("bad url %q from %q", u, out)
			}
		}
	})
}

func FuzzURLAndNotFound(f *testing.F) {
	for _, s := range []string{
		"", "\n", "git@github.com:a/b.git\n", "Could not resolve to a Repository with the name",
		"gh: Not Found (HTTP 404)", "HTTP 403", "\x1b]52;c;x\x07", "a\nb",
	} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, out string) {
		g := &Client{Run: func(_ context.Context, _ ...string) ([]byte, error) { return []byte(out), nil }}
		url, err := g.url(context.Background(), "creds-vault")
		switch {
		case err != nil:
		case url == "" || url != strings.TrimSpace(url):
			t.Fatalf("bad url %q from %q", url, out)
		}
		failing := &Client{Run: func(_ context.Context, _ ...string) ([]byte, error) { return nil, errors.New(out) }}
		exists, err := failing.RepoExists(context.Background(), "creds-vault")
		if exists {
			t.Fatalf("exists on failure, out %q", out)
		}
		if err == nil && !notFound(errors.New(out)) {
			t.Fatalf("swallowed error %q", out)
		}
	})
}

func FuzzGhError(f *testing.F) {
	f.Add("repo view x", "gh: bad credentials")
	f.Add("", "")
	f.Add("a\x00b", "\x1b]52;c;x\x07\nnot found")
	f.Fuzz(func(t *testing.T, arg, stderr string) {
		base := errors.New("exit status 1")
		err := ghError([]string{arg}, stderr, base)
		if err == nil {
			t.Fatal("nil error")
		}
		if strings.TrimSpace(stderr) == "" && !errors.Is(err, base) {
			t.Fatalf("lost base error for %q", stderr)
		}
	})
}
