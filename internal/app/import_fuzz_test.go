package app

import (
	"errors"
	"strings"
	"testing"
)

func FuzzDecodeImport(f *testing.F) {
	for _, s := range []string{
		"",
		"{}",
		"null",
		`{"format":"creds","version":1,"entries":[]}`,
		`{"format":"creds","version":1,"entries":[{"path":"a/b","type":"login","fields":[{"name":"x","value":"=cmd","secret":true}]}]}`,
		`{"format":"creds","version":1,"entries":[{"path":"../../\u0000","type":"nope"}]}`,
		`{"format":"creds","version":1,"entries":null,"extra":1}`,
		`{"format":"creds","version":1e400}`,
		`{"format":"creds","format":"creds","version":1}`,
		`{"format":"日本","version":-1}`,
		`[{"path":"a"}]`,
		"\xff\xfe",
		`{"format":"creds","version":1,"entries":[` + strings.Repeat(`{"path":"p","type":"note"},`, 200) + `{}]}`,
	} {
		f.Add([]byte(s))
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		entries, err := decodeImport(strings.NewReader(string(data)))
		if err != nil {
			if entries != nil {
				t.Fatalf("entries on error: %v", err)
			}
			if !errors.Is(err, ErrImportFile) {
				t.Fatalf("unwrapped error: %v", err)
			}
		}
	})
}
