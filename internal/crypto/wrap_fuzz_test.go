package crypto

import (
	"strings"
	"testing"
)

func FuzzNormalizeRecoveryCode(f *testing.F) {
	for _, s := range []string{"", "abcde-fghij", " A B\tC\r\nD ", "ß", "ǆ", "日本-語", "\x00", "\xff\xfe", strings.Repeat("-", 100)} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		got := NormalizeRecoveryCode(s)
		if strings.ContainsAny(got, "- \t\r\n") {
			t.Fatalf("separator kept: %q", got)
		}
		if again := NormalizeRecoveryCode(got); again != got {
			t.Fatalf("not idempotent: %q -> %q", got, again)
		}
	})
}

func FuzzUnwrapIdentity(f *testing.F) {
	id, err := NewIdentity()
	if err != nil {
		f.Fatal(err)
	}
	wrapped, err := WrapIdentity(id, testSecret, testLogN)
	if err != nil {
		f.Fatal(err)
	}
	f.Add(wrapped, testSecret)
	f.Add(wrapped, "")
	f.Add([]byte{}, testSecret)
	f.Add([]byte("age-encryption.org/v1\n-> scrypt AAAA 99\n\n--- AAAA\n"), testSecret)
	f.Add([]byte("age-encryption.org/v1\n"), "x")
	f.Add([]byte("\x00\xff"), "\x00")
	f.Add(Pad(nil), testSecret)
	f.Fuzz(func(t *testing.T, data []byte, secret string) {
		got, err := UnwrapIdentity(data, secret)
		if err != nil {
			if got != nil {
				t.Fatalf("identity on error: %v", err)
			}
			return
		}
		if got.String() == "" {
			t.Fatal("empty identity")
		}
	})
}
