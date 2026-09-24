package device

import (
	"errors"
	"strings"
	"testing"
)

func TestParseKey(t *testing.T) {
	t.Parallel()
	cases := map[string]struct{ text, id, rec string }{
		"se": {
			text: "# created: 2026-01-01T00:00:00Z\n# access control: current biometry\n# public key: age1se1abc\nAGE-PLUGIN-SE-1XYZ\n\n",
			id:   "AGE-PLUGIN-SE-1XYZ", rec: "age1se1abc",
		},
		"yubikey": {
			text: "#       Serial: 1, Slot: 1\n#    Recipient: age1yubikey1abc\nAGE-PLUGIN-YUBIKEY-1XYZ\n",
			id:   "AGE-PLUGIN-YUBIKEY-1XYZ", rec: "age1yubikey1abc",
		},
		"crlf": {
			text: "# Recipient: age1tpm1abc\r\nAGE-PLUGIN-TPM-1XYZ\r\n",
			id:   "AGE-PLUGIN-TPM-1XYZ", rec: "age1tpm1abc",
		},
		"no recipient": {text: "AGE-PLUGIN-SE-1XYZ", id: "AGE-PLUGIN-SE-1XYZ"},
		"pq comment skipped": {
			text: "# public key: age1se1abc\n# public key (post-quantum): age1tagpq1abc\nAGE-PLUGIN-SE-1XYZ\n",
			id:   "AGE-PLUGIN-SE-1XYZ", rec: "age1se1abc",
		},
	}
	for name, tc := range cases {
		id, rec, err := ParseKey(tc.text)
		if err != nil || id != tc.id || rec != tc.rec {
			t.Errorf("%s: got %q %q %v", name, id, rec, err)
		}
	}
}

func TestParseKeyRefuses(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"empty":       "",
		"comments":    "# public key: age1se1abc\n",
		"native key":  "AGE-SECRET-KEY-1XYZ\n",
		"two plugins": "AGE-PLUGIN-SE-1XYZ\nAGE-PLUGIN-SE-1ABC\n",
		"junk":        "AGE-PLUGIN-SE-1XYZ\nhello\n",
		"huge":        "AGE-PLUGIN-SE-1XYZ\n" + strings.Repeat("#", maxKeySize),
	}
	for name, text := range cases {
		if _, _, err := ParseKey(text); !errors.Is(err, ErrBadKey) {
			t.Errorf("%s: got %v", name, err)
		}
	}
}
