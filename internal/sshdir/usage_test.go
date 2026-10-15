package sshdir

import (
	"errors"
	"maps"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"
)

type usageCase struct {
	name    string
	configs map[string]string
	keys    []string
	used    []string
	unused  []string
	missing []string
	warns   []error
}

var includeTree = map[string]string{
	"config.d/a":      "IdentityFile a_key\n",
	"config.d/b":      "IdentityFile b_key\n",
	"config.d/deep/c": "IdentityFile c_key\n",
}

func withConfig(top string) map[string]string {
	m := map[string]string{"config": top}
	maps.Copy(m, includeTree)
	return m
}

var usageCases = []usageCase{
	{
		name:   "defaults without config",
		keys:   []string{"id_rsa", "id_ed25519", "id_ecdsa_sk", "work"},
		used:   []string{"id_ecdsa_sk", "id_ed25519", "id_rsa"},
		unused: []string{"work"},
	},
	{
		name: "identity forms",
		configs: map[string]string{"config": "# IdentityFile hidden\nHost a\n  IdentityFile ~/.ssh/work\n" +
			"Host b\n  identityfile=k_eq\n  IDENTITYFILE = \"with space\"\n  IdentityFile\tk_tab # note\n" +
			"  IdentityFile {dir}/k_abs\n  IdentityFile ~/.ssh/sub/k_sub\n  IdentityFile none\n" +
			"Match host c\n  IdentityFile 'k_single'\n  IdentityFile k\\ esc\n"},
		keys:   []string{"work", "k_eq", "with space", "k_tab", "k_abs", "sub/k_sub", "k_single", "k esc", "hidden"},
		used:   []string{"k esc", "k_abs", "k_eq", "k_single", "k_tab", "sub/k_sub", "with space", "work"},
		unused: []string{"hidden"},
	},
	{
		name:    "include glob",
		configs: withConfig("Include config.d/*\n"),
		keys:    []string{"a_key", "b_key", "c_key"},
		used:    []string{"a_key", "b_key"},
		unused:  []string{"c_key"},
	},
	{
		name:    "include several patterns with tilde and quotes",
		configs: withConfig("Include ~/.ssh/config.d/a \"config.d/deep/*\"\n"),
		keys:    []string{"a_key", "b_key", "c_key"},
		used:    []string{"a_key", "c_key"},
		unused:  []string{"b_key"},
	},
	{
		name:    "include absolute inside dir",
		configs: withConfig("Host x\n  Include {dir}/config.d/b\n"),
		keys:    []string{"a_key", "b_key"},
		used:    []string{"b_key"},
		unused:  []string{"a_key"},
	},
	{
		name:    "unreached include file ignored",
		configs: withConfig("Include config.d/none\n"),
		keys:    []string{"a_key"},
		unused:  []string{"a_key"},
	},
	{
		name:    "tokens warn and skip",
		configs: withConfig("IdentityFile ~/.ssh/%h\nIdentityFile ${HOME}/.ssh/k\nInclude config.d/%h\n"),
		keys:    []string{"a_key"},
		unused:  []string{"a_key"},
		warns:   []error{ErrToken, ErrToken, ErrToken},
	},
	{
		name: "include loop",
		configs: map[string]string{
			"config":     "Include config.d/a\n",
			"config.d/a": "Include config config.d/a\nIdentityFile a_key\n",
		},
		keys:  []string{"a_key"},
		used:  []string{"a_key"},
		warns: []error{ErrIncludeLoop, ErrIncludeLoop},
	},
	{
		name: "outside the ssh dir",
		configs: map[string]string{"config": "IdentityFile /elsewhere/k\nIdentityFile ~/k\n" +
			"IdentityFile ~other/k\nInclude ../x\nIdentityFile {dir}/../k\n"},
		warns: []error{ErrOutside, ErrOutside, ErrOutside, ErrOutside, ErrOutside},
	},
	{
		name: "missing targets",
		configs: map[string]string{"config": "IdentityFile gone\nIdentityFile ~/.ssh/gone\n" +
			"IdentityFile agent.pub\nIdentityFile linked\nIdentityFile id_rsa\n"},
		missing: []string{"gone", "id_rsa"},
	},
	{
		name:    "syntax errors",
		configs: map[string]string{"config": "IdentityFile \"open\nIdentityFile\nInclude [\nIdentityFile \"\"\n"},
		warns:   []error{ErrSyntax, ErrSyntax, ErrSyntax, ErrSyntax},
	},
	{
		name:    "crlf and keyword case",
		configs: map[string]string{"config": "INCLUDE config.d/a\r\n", "config.d/a": "IdentityFile a_key\r\n"},
		keys:    []string{"a_key"},
		used:    []string{"a_key"},
	},
}

func fakeResult(configs map[string]string, keys []string, dir string) Result {
	var res Result
	for rel, text := range configs {
		text = strings.ReplaceAll(text, "{dir}", filepath.ToSlash(dir))
		res.Files = append(res.Files, File{Rel: rel, Kind: KindConfig, Data: []byte(text)})
	}
	for _, rel := range keys {
		res.Files = append(res.Files, File{Rel: rel, Kind: KindPrivateKey})
	}
	res.Files = append(res.Files, File{Rel: "agent.pub", Kind: KindPublicKey})
	res.Skipped = append(res.Skipped, Skip{Rel: "linked", Err: ErrSymlink})
	sort.Slice(res.Files, func(i, j int) bool { return res.Files[i].Rel < res.Files[j].Rel })
	return res
}

func rels(files []File) []string {
	out := []string{}
	for _, f := range files {
		out = append(out, f.Rel)
	}
	sort.Strings(out)
	return out
}

func TestReadUsage(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, ".ssh")
	for _, tc := range usageCases {
		t.Run(tc.name, func(t *testing.T) {
			u := ReadUsage(fakeResult(tc.configs, tc.keys, dir), dir, home)
			checkList(t, "used", rels(u.Used), tc.used)
			checkList(t, "unused", rels(u.Unused), tc.unused)
			checkList(t, "missing", u.Missing, tc.missing)
			checkWarns(t, u.Warnings, tc.warns)
		})
	}
}

func checkList(t *testing.T, what string, got, want []string) {
	t.Helper()
	if len(got) == 0 && len(want) == 0 {
		return
	}
	if !slices.Equal(got, want) {
		t.Errorf("%s = %q, want %q", what, got, want)
	}
}

func checkWarns(t *testing.T, got, want []error) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("warnings = %v, want %v", got, want)
	}
	for i := range want {
		if !errors.Is(got[i], want[i]) {
			t.Errorf("warning %d = %v, want %v", i, got[i], want[i])
		}
	}
}

func TestReadUsageWarningNamesFileAndLine(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, ".ssh")
	res := fakeResult(map[string]string{"config": "Host a\n  IdentityFile ~/.ssh/%h\n"}, nil, dir)
	u := ReadUsage(res, dir, home)
	if len(u.Warnings) != 1 || !strings.HasPrefix(u.Warnings[0].Error(), "config:2: ~/.ssh/%h: ") {
		t.Fatalf("warnings = %v", u.Warnings)
	}
}

func TestReadUsageOverScan(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, ".ssh")
	ed := openssh(t, ed25519Key(t))
	writeFiles(t, dir, map[string][]byte{
		"config":                 []byte("Include config.d/*\nIdentityFile ~/.ssh/work.pem\nIdentityFile ~/.ssh/gone\n"),
		"config.d/hosts":         []byte("Host beam\n  IdentityFile ~/.ssh/config.d/beam/data_new\n"),
		"config.d/beam/data_new": ed,
		"id_ed25519":             ed,
		"work.pem":               pkcs1(rsaKey(t)),
		"spare":                  ed,
		"known_hosts":            []byte("host ssh-ed25519 AAAA\n"),
	})
	res, err := Scan(dir)
	if err != nil {
		t.Fatal(err)
	}
	u := ReadUsage(res, dir, home)
	checkList(t, "used", rels(u.Used), []string{"config.d/beam/data_new", "id_ed25519", "work.pem"})
	checkList(t, "unused", rels(u.Unused), []string{"spare"})
	checkList(t, "missing", u.Missing, []string{"gone"})
	checkWarns(t, u.Warnings, nil)
}
