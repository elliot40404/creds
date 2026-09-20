package vault

import (
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/elliot40404/creds/internal/safetext"
)

func FuzzValidatePath(f *testing.F) {
	for _, s := range []string{
		"", "a", "a/b", "/a", "-rf", "a//b", "a/./b", "a/../b", "..", ".", " a", "a ", "a/ b",
		"a\x00b", "a\x1bb", "a\nb", "a\u2028b", "ünï/kèy", strings.Repeat("a/", 500) + "b",
		"C:/a", "\\a", "a\tb",
	} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, path string) {
		if err := ValidatePath(path); err != nil {
			return
		}
		switch {
		case path == "":
			t.Fatal("accepted empty path")
		case strings.HasPrefix(path, "-"), strings.HasPrefix(path, "/"):
			t.Fatalf("accepted %q", path)
		}
		for seg := range strings.SplitSeq(path, "/") {
			if seg == "" || seg == "." || seg == ".." || seg != strings.TrimSpace(seg) {
				t.Fatalf("accepted %q with segment %q", path, seg)
			}
		}
	})
}

func FuzzEntryValidate(f *testing.F) {
	f.Add("a/b", "name", "login", "field")
	f.Add("", "", "", "")
	f.Add("a", "n\x1b]52;c;x\x07", "login", "f")
	f.Add("a", "n", "nope", "f")
	f.Add("a", "n", "login", "f\nx")
	f.Add("-a", "n", "login", "")
	f.Fuzz(func(t *testing.T, path, name, typ, field string) {
		e := Entry{
			ID:      uuid.NewV7(),
			Path:    path,
			Name:    name,
			Type:    Type(typ),
			Fields:  []Field{{Name: field, Value: "v"}},
			Created: time.Unix(0, 0).UTC(),
			Updated: time.Unix(0, 0).UTC(),
		}
		if err := e.Validate(); err != nil {
			return
		}
		bad := path == "" || field == "" || !Type(typ).Valid()
		bad = bad || safetext.HasControl(path) || safetext.HasControl(name) || safetext.HasControl(field)
		if bad {
			t.Fatalf("accepted entry path %q name %q type %q field %q", path, name, typ, field)
		}
	})
}
