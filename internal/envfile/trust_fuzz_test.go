package envfile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"
)

func FuzzLoadTrust(f *testing.F) {
	for _, s := range []string{
		"", "{}", `{"version":1,"projects":{}}`, `{"version":1,"projects":null}`,
		`{"version":2,"projects":{}}`, `{"version":1}`, `{"version":1,"projects":{"a":"b"},"x":1}`,
		`{"version":1,"projects":{"/a/.creds.toml":"` + strings.Repeat("0", 64) + `"}}`,
		`{"version":1.5,"projects":{}}`, `{"version":1,"projects":{"a":1}}`,
		"\x00", "[]", `{"version":1,"projects":{"a":"b"},"projects":{"a":"c"}}`,
		"{\"version\":1,\"projects\":{\"\u001b]52;c;x\u0007\":\"b\"}}",
	} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, src string) {
		store := filepath.Join(t.TempDir(), "trust.json")
		if err := os.WriteFile(store, []byte(src), 0o600); err != nil {
			t.Fatal(err)
		}
		got, err := loadTrust(store)
		if err != nil {
			return
		}
		if got.Version != trustVersion {
			t.Fatalf("version %d accepted", got.Version)
		}
		if got.Projects == nil {
			t.Fatal("nil projects map")
		}
		p := Project{File: "/p/.creds.toml", Sum: sum(src)}
		if err := Trust(store, p); err != nil {
			t.Fatalf("trust: %v", err)
		}
		ok, err := Trusted(store, p)
		if err != nil || !ok {
			t.Fatalf("trusted after trust = %v, %v", ok, err)
		}
	})
}

func FuzzTrustedSum(f *testing.F) {
	f.Add("", "")
	f.Add("/a/.creds.toml", "x")
	f.Add("a\x00b", strings.Repeat("f", 64))
	f.Fuzz(func(t *testing.T, file, s string) {
		if !utf8.ValidString(file) || !utf8.ValidString(s) {
			return
		}
		store := filepath.Join(t.TempDir(), "trust.json")
		p := Project{File: file, Sum: s}
		ok, err := Trusted(store, p)
		if err != nil {
			t.Fatalf("trusted: %v", err)
		}
		if ok {
			t.Fatalf("trusted with no store: %q %q", file, s)
		}
		if err := Trust(store, p); err != nil {
			t.Fatalf("trust: %v", err)
		}
		ok, err = Trusted(store, p)
		if err != nil {
			t.Fatalf("trusted after: %v", err)
		}
		if ok != (s != "") {
			t.Fatalf("trusted = %v for sum %q", ok, s)
		}
	})
}
