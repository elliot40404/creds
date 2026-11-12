package gitsync

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const fetchLine = "\tfetch = +refs/heads/*:refs/remotes/origin/*\n"

var originCases = []struct {
	name string
	conf string
	want originState
	url  string
}{
	{"plain", "[core]\n\tbare = false\n[remote \"origin\"]\n\turl = /srv/vault.git\n" + fetchLine, originFound, "/srv/vault.git"},
	{"escaped backslash", "[remote \"origin\"]\n\turl = C:\\\\x\\\\remote.git\n", originFound, `C:\x\remote.git`},
	{"case insensitive names", "[Remote \"origin\"]\n\tURL = a\n", originFound, "a"},
	{"spaces around value", "[remote \"origin\"]\n\turl =   a  \n", originFound, "a"},
	{"tabs around equals", "[remote \"origin\"]\n\turl\t=\ta\n", originFound, "a"},
	{"comments", "# top\n; semi\n[remote \"origin\"] # c\n\t# inner\n\turl = git@host:me/v.git\n", originFound, "git@host:me/v.git"},
	{"other remote", "[remote \"backup\"]\n\turl = b\n[remote \"origin\"]\n\turl = a\n", originFound, "a"},
	{"no remote", "[core]\n\tbare = false\n", originNone, ""},
	{"empty file", "", originNone, ""},
	{"subsection case sensitive", "[remote \"Origin\"]\n\turl = a\n", originNone, ""},
	{"two urls", "[remote \"origin\"]\n\turl = a\n\turl = b\n", originUnsure, ""},
	{"two sections", "[remote \"origin\"]\n\turl = a\n[remote \"origin\"]\n\tpushurl = b\n", originUnsure, ""},
	{"no url", "[remote \"origin\"]\n" + fetchLine, originUnsure, ""},
	{"include", "[include]\n\tpath = x\n[remote \"origin\"]\n\turl = a\n", originUnsure, ""},
	{"include upper", "[INCLUDE]\n\tpath = x\n[remote \"origin\"]\n\turl = a\n", originUnsure, ""},
	{"includeIf", "[includeIf \"gitdir:/x/\"]\n\tpath = x\n[remote \"origin\"]\n\turl = a\n", originUnsure, ""},
	{"extensions", "[extensions]\n\tworktreeConfig = true\n[remote \"origin\"]\n\turl = a\n", originUnsure, ""},
	{"old style section", "[remote.origin]\n\turl = a\n", originUnsure, ""},
	{"key on header line", "[remote \"origin\"] url = a\n", originUnsure, ""},
	{"double space header", "[remote  \"origin\"]\n\turl = a\n", originUnsure, ""},
	{"bracket in subsection", "[remote \"a]b\"]\n\turl = x\n[remote \"origin\"]\n\turl = a\n", originUnsure, ""},
	{"quoted value", "[remote \"origin\"]\n\turl = \"a\"\n", originUnsure, ""},
	{"inline hash", "[remote \"origin\"]\n\turl = a # b\n", originUnsure, ""},
	{"inline semicolon", "[remote \"origin\"]\n\turl = a;b\n", originUnsure, ""},
	{"inner space", "[remote \"origin\"]\n\turl = a b\n", originUnsure, ""},
	{"escape tab", "[remote \"origin\"]\n\turl = a\\tb\n", originUnsure, ""},
	{"trailing backslash", "[remote \"origin\"]\n\turl = a\\\n", originUnsure, ""},
	{"continuation hides section", "[core]\n\tx = a \\\n[remote \"origin\"]\n\turl = evil\n", originUnsure, ""},
	{"unclosed quote before", "[core]\n\tx = \"a\n[remote \"origin\"]\n\turl = a\n", originUnsure, ""},
	{"no value", "[remote \"origin\"]\n\turl\n", originUnsure, ""},
	{"empty value", "[remote \"origin\"]\n\turl =\n", originUnsure, ""},
	{"key before section", "url = a\n[remote \"origin\"]\n\turl = a\n", originUnsure, ""},
	{"crlf", "[remote \"origin\"]\r\n\turl = a\r\n", originUnsure, ""},
	{"bom", "\xef\xbb\xbf[remote \"origin\"]\n\turl = a\n", originUnsure, ""},
	{"nul", "[remote \"origin\"]\n\turl = a\x00b\n", originUnsure, ""},
	{"control char", "[remote \"origin\"]\n\turl = a\x01b\n", originUnsure, ""},
	{"too large", strings.Repeat("# pad\n", maxConfigSize/6+1) + "[remote \"origin\"]\n\turl = a\n", originUnsure, ""},
}

func TestReadOriginMatchesGit(t *testing.T) {
	t.Parallel()
	for _, tc := range originCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			r := newRepo(t)
			if err := os.WriteFile(filepath.Join(r.Dir, ".git", "config"), []byte(tc.conf), 0o600); err != nil {
				t.Fatal(err)
			}
			got, st := readOrigin(r.Dir)
			if st != tc.want || got != tc.url {
				t.Fatalf("readOrigin = %q %d, want %q %d", got, st, tc.url, tc.want)
			}
			gitURL, gitErr := r.gitRemoteURL(context.Background())
			if st != originUnsure && (gitErr != nil || gitURL != got) {
				t.Fatalf("git says %q %v, parser %q", gitURL, gitErr, got)
			}
			if has := HasRemote(context.Background(), r.Dir); has != (gitErr == nil && gitURL != "") {
				t.Fatalf("HasRemote %v, git %q %v", has, gitURL, gitErr)
			}
		})
	}
}

func TestReadOriginMissing(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if _, st := readOrigin(dir); st != originNone {
		t.Fatalf("state %d", st)
	}
	if HasRemote(context.Background(), dir) {
		t.Fatal("HasRemote on a dir without git")
	}
}

func TestReadOriginRefusesLinkedConfig(t *testing.T) {
	t.Parallel()
	r := newRepo(t)
	path := filepath.Join(r.Dir, ".git", "config")
	target := filepath.Join(t.TempDir(), "config")
	if err := os.WriteFile(target, []byte("[remote \"origin\"]\n\turl = a\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, path); err != nil {
		t.Skipf("no symlinks: %v", err)
	}
	if _, st := readOrigin(r.Dir); st != originUnsure {
		t.Fatalf("state %d", st)
	}
}
