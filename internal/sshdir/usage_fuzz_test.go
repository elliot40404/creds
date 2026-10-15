package sshdir

import (
	"path"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func FuzzParseConfig(f *testing.F) {
	for _, s := range []string{
		"", "Include config.d/*\n", "IdentityFile ~/.ssh/id\n", "IdentityFile=\"a b\" # c\n",
		"Include config\n", "Include ../x [\n", "IdentityFile %d/%h\n", "IdentityFile \"open\n",
		"identityfile ~\nInclude ~\\x\n", "IdentityFile k\\ \\\"q\\\\\n", "Include /etc/*\r\n",
	} {
		f.Add(s, "Include config.d/*\nIdentityFile b\n")
	}
	home := f.TempDir()
	dir := filepath.Join(home, ".ssh")
	f.Fuzz(func(t *testing.T, top, inc string) {
		res := Result{Files: []File{
			{Rel: "b", Kind: KindPrivateKey},
			{Rel: "config", Kind: KindConfig, Data: []byte(top)},
			{Rel: "config.d/a", Kind: KindConfig, Data: []byte(inc)},
			{Rel: "id_rsa", Kind: KindPrivateKey},
		}}
		u := ReadUsage(res, dir, home)
		if len(u.Used)+len(u.Unused) != 2 {
			t.Fatalf("used %d + unused %d keys, want 2", len(u.Used), len(u.Unused))
		}
		if !slices.ContainsFunc(u.Used, func(f File) bool { return f.Rel == "id_rsa" }) {
			t.Fatal("default key not used")
		}
		checkMissing(t, u.Missing)
	})
}

func checkMissing(t *testing.T, missing []string) {
	t.Helper()
	seen := map[string]bool{}
	for _, rel := range missing {
		if seen[rel] || rel == ".." || strings.HasPrefix(rel, "../") || path.IsAbs(rel) || filepath.IsAbs(rel) {
			t.Fatalf("bad missing path %q in %q", rel, missing)
		}
		seen[rel] = true
	}
}
