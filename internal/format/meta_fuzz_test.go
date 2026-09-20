package format

import (
	"os"
	"path/filepath"
	"testing"
)

func FuzzLoadMeta(f *testing.F) {
	for _, s := range []string{
		"",
		"{}",
		"null",
		`{"format_version":1,"recipient":"age1x","created":"2026-01-01T00:00:00Z"}`,
		`{"format_version":1,"recipient":"age1x","created":"2026-01-01T00:00:00Z","x":1}`,
		`{"format_version":2}`,
		`{"format_version":-1}`,
		`{"format_version":1e400}`,
		`{"format_version":"1"}`,
		`{"format_version":1,"format_version":1}`,
		`{"format_version":1,"recipient":"../\u0000=cmd"}`,
		"\xff",
	} {
		f.Add([]byte(s))
	}
	path := filepath.Join(f.TempDir(), "meta.json")
	f.Fuzz(func(t *testing.T, data []byte) {
		if err := os.WriteFile(path, data, 0o600); err != nil {
			t.Fatal(err)
		}
		m, err := LoadMeta(path)
		if err != nil {
			return
		}
		if m.FormatVersion != CurrentVersion {
			t.Fatalf("version %d accepted", m.FormatVersion)
		}
		if err := SaveMeta(path, m); err != nil {
			t.Fatal(err)
		}
		again, err := LoadMeta(path)
		if err != nil || again.FormatVersion != m.FormatVersion || again.Recipient != m.Recipient || !again.Created.Equal(m.Created) {
			t.Fatalf("roundtrip: %+v %+v %v", m, again, err)
		}
	})
}
