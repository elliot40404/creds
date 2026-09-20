package app

import (
	_ "embed"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	minPasswordLen   = 12
	passphraseLen    = 20
	minRunLen        = 4
	minUniqueRunes   = 5
	minLeftoverRunes = 6
)

var ErrShortPassword = fmt.Errorf("password must be at least %d characters", minPasswordLen)

//go:embed common.txt
var commonList string

var leet = strings.NewReplacer("0", "o", "1", "i", "3", "e", "4", "a", "5", "s", "7", "t", "@", "a", "$", "s")

func checkPassword(pw string) (string, error) {
	if utf8.RuneCountInString(pw) < minPasswordLen {
		return "", ErrShortPassword
	}
	switch {
	case strings.Contains(strings.ToLower(pw), "creds"):
		return "contains the word creds", nil
	case hasCommonWord(pw):
		return "built on a common password", nil
	case uniqueRunes(pw) < minUniqueRunes:
		return "too few different characters", nil
	case hasRun(pw):
		return "has repeats or sequences like aaaa or 1234", nil
	case utf8.RuneCountInString(pw) < passphraseLen && charClasses(pw) == 1:
		return "uses only one kind of character", nil
	}
	return "", nil
}

func hasCommonWord(pw string) bool {
	norm := leet.Replace(strings.ToLower(pw))
	for word := range strings.FieldsSeq(commonList) {
		if !strings.Contains(norm, word) {
			continue
		}
		rest := strings.ReplaceAll(norm, word, "")
		if utf8.RuneCountInString(rest) < minLeftoverRunes {
			return true
		}
	}
	return false
}

func uniqueRunes(pw string) int {
	seen := map[rune]struct{}{}
	for _, r := range pw {
		seen[r] = struct{}{}
	}
	return len(seen)
}

func hasRun(pw string) bool {
	rs := []rune(strings.ToLower(pw))
	same, up, down := 1, 1, 1
	for i := 1; i < len(rs); i++ {
		d := rs[i] - rs[i-1]
		same = step(same, d == 0)
		up = step(up, d == 1)
		down = step(down, d == -1)
		if same >= minRunLen || up >= minRunLen || down >= minRunLen {
			return true
		}
	}
	return false
}

func step(n int, ok bool) int {
	if ok {
		return n + 1
	}
	return 1
}

func charClasses(pw string) int {
	var lower, upper, digit, other int
	for _, r := range pw {
		switch {
		case unicode.IsLower(r):
			lower = 1
		case unicode.IsUpper(r):
			upper = 1
		case unicode.IsDigit(r):
			digit = 1
		default:
			other = 1
		}
	}
	return lower + upper + digit + other
}
