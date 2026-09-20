package gh

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
)

var errNoTool = errors.New("not found")

type fakeGH struct {
	replies map[string]string
	fails   map[string]bool
	errs    map[string]string
	calls   []string
	missing bool
}

func (f *fakeGH) client() *Client {
	return &Client{
		Look: func(file string) (string, error) {
			if f.missing {
				return "", errNoTool
			}
			return "/usr/bin/" + file, nil
		},
		Run: func(_ context.Context, args ...string) ([]byte, error) {
			key := strings.Join(args, " ")
			f.calls = append(f.calls, key)
			if f.fails[key] {
				msg := f.errs[key]
				if msg == "" {
					msg = "gh failed: " + key
				}
				return nil, errors.New(msg)
			}
			return []byte(f.replies[key]), nil
		},
	}
}

const (
	viewName = "repo view creds-vault --json name"
	viewURL  = "repo view creds-vault --json sshUrl --jq .sshUrl"
	createIt = "repo create creds-vault --private"
)

func newFakeGH() *fakeGH {
	return &fakeGH{
		replies: map[string]string{viewURL: "git@github.com:me/creds-vault.git\n"},
		fails:   map[string]bool{viewName: true},
		errs:    map[string]string{viewName: "gh repo view: GraphQL: Could not resolve to a Repository with the name 'me/creds-vault'."},
	}
}

func TestGHAvailable(t *testing.T) {
	t.Parallel()
	f := newFakeGH()
	if err := f.client().Available(context.Background()); err != nil {
		t.Fatalf("available: %v", err)
	}
	f.missing = true
	if err := f.client().Available(context.Background()); !errors.Is(err, ErrNoHost) {
		t.Fatalf("missing gh: %v", err)
	}
	f = newFakeGH()
	f.fails["auth status"] = true
	if err := f.client().Available(context.Background()); !errors.Is(err, ErrNoHost) {
		t.Fatalf("logged out: %v", err)
	}
}

func TestGHCreate(t *testing.T) {
	t.Parallel()
	f := newFakeGH()
	url, err := f.client().Create(context.Background(), "creds-vault")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if url != "git@github.com:me/creds-vault.git" {
		t.Fatalf("url = %q", url)
	}
	if !slices.Contains(f.calls, createIt) {
		t.Fatalf("calls = %v", f.calls)
	}
}

func TestGHCreateRefusesExisting(t *testing.T) {
	t.Parallel()
	f := newFakeGH()
	f.fails[viewName] = false
	f.replies[viewName] = `{"name":"creds-vault"}`
	if _, err := f.client().Create(context.Background(), "creds-vault"); !errors.Is(err, ErrRepoExists) {
		t.Fatalf("err = %v", err)
	}
	if slices.Contains(f.calls, createIt) {
		t.Fatal("created an existing repo")
	}
}

func TestGHCreateRejectsBadNames(t *testing.T) {
	t.Parallel()
	f := newFakeGH()
	for _, name := range []string{"", "-x", "a b", "a/b", "a;b", strings.Repeat("a", 101)} {
		if _, err := f.client().Create(context.Background(), name); !errors.Is(err, ErrBadRepoName) {
			t.Fatalf("%q: err = %v", name, err)
		}
	}
	if len(f.calls) != 0 {
		t.Fatalf("ran gh for a bad name: %v", f.calls)
	}
}

func TestGHRepos(t *testing.T) {
	t.Parallel()
	f := newFakeGH()
	key := "repo list --limit 100 --json sshUrl --jq .[].sshUrl"
	f.replies[key] = "git@github.com:me/a.git\ngit@github.com:me/b.git\n"
	urls, err := f.client().Repos(context.Background())
	if err != nil {
		t.Fatalf("repos: %v", err)
	}
	if len(urls) != 2 || urls[1] != "git@github.com:me/b.git" {
		t.Fatalf("urls = %v", urls)
	}
}

func TestGHRepoExistsReportsRealFailures(t *testing.T) {
	t.Parallel()
	f := newFakeGH()
	f.errs[viewName] = "gh repo view: dial tcp: lookup api.github.com: no such host"
	exists, err := f.client().RepoExists(context.Background(), "creds-vault")
	if err == nil || exists {
		t.Fatalf("exists = %v, err = %v", exists, err)
	}
	if _, err := f.client().Create(context.Background(), "creds-vault"); err == nil {
		t.Fatal("created a repo after a failed check")
	}
	if slices.Contains(f.calls, createIt) {
		t.Fatalf("calls = %v", f.calls)
	}
}

func TestGHRepoExistsFreeName(t *testing.T) {
	t.Parallel()
	f := newFakeGH()
	exists, err := f.client().RepoExists(context.Background(), "creds-vault")
	if err != nil || exists {
		t.Fatalf("exists = %v, err = %v", exists, err)
	}
}

func TestRepoExistsOnlyTrustsTheGHNotFoundError(t *testing.T) {
	t.Parallel()
	f := newFakeGH()
	f.errs[viewName] = `exec: "gh": executable file not found in $PATH`
	if exists, err := f.client().RepoExists(context.Background(), "creds-vault"); err == nil {
		t.Fatalf("exists %v with no error", exists)
	}
	f = newFakeGH()
	if exists, err := f.client().RepoExists(context.Background(), "creds-vault"); err != nil || exists {
		t.Fatalf("exists %v err %v", exists, err)
	}
}
