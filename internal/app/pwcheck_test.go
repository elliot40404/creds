package app

import (
	"errors"
	"testing"
)

func TestCheckPassword(t *testing.T) {
	t.Parallel()
	tests := []struct {
		pw   string
		weak bool
	}{
		{"Tr0ub4dor&3x", false},
		{"correct horse battery staple", false},
		{"xkqwpvmzjrtlnbyd", true},
		{"lowercaseonly", true},
		{"481739265017", true},
		{"P@ssw0rd2024!", true},
		{"Qwerty!98765x", true},
		{"MyCredsVault#9", true},
		{"aaaaXy7!kPq2", true},
		{"zq1234Xy!kPm", true},
		{"zqDCBAy!kPm9", true},
		{"aB1aB1aB1aB1", true},
		{"Password is my dog Rex 42", false},
	}
	for _, tt := range tests {
		reason, err := checkPassword(tt.pw)
		if err != nil {
			t.Fatalf("%q: %v", tt.pw, err)
		}
		if (reason != "") != tt.weak {
			t.Errorf("%q: reason %q, want weak=%v", tt.pw, reason, tt.weak)
		}
	}
}

func TestCheckPasswordShort(t *testing.T) {
	t.Parallel()
	for _, pw := range []string{"", "Xy7!kPq2mZ4", "ÄÖÜäöüßÄÖÜä"} {
		if _, err := checkPassword(pw); !errors.Is(err, ErrShortPassword) {
			t.Errorf("%q: err %v", pw, err)
		}
	}
}
