package search

import (
	"unicode"

	"github.com/junegunn/fzf/src/util"
)

const (
	maxWord  = 32
	typoMin  = 4
	typoLong = 8
)

func (t *term) typos() int {
	switch {
	case !t.fuzzy || t.not || len(t.text) < typoMin || len(t.text) > maxWord:
		return 0
	case len(t.text) >= typoLong:
		return 2
	}
	return 1
}

func (t *term) typo(c *util.Chars) (int, int, bool) {
	k := t.typos()
	if k == 0 {
		return 0, 0, false
	}
	var word [maxWord]rune
	n, start, size := 0, 0, c.Length()
	for i := 0; i <= size; i++ {
		if i < size {
			if r := c.Get(i); unicode.IsLetter(r) || unicode.IsDigit(r) {
				if n == 0 {
					start = i
				}
				if n < maxWord {
					word[n] = t.fold1(r)
				}
				n++
				continue
			}
		}
		if n > 0 && near(t.text, word[:min(n, maxWord)], k) {
			return start, start + n, true
		}
		n = 0
	}
	return 0, 0, false
}

func (t *term) fold1(r rune) rune {
	if t.fold {
		return unicode.ToLower(r)
	}
	return r
}

func near(a, b []rune, k int) bool {
	if len(b) < len(a)-k {
		return false
	}
	var rows [3][maxWord + 1]int
	prev2, prev, cur := rows[0][:], rows[1][:], rows[2][:]
	for j := range len(b) + 1 {
		prev[j] = j
	}
	for i := 1; i <= len(a); i++ {
		ai, before, last := a[i-1], rune(-1), rune(-1)
		if i > 1 {
			before = a[i-2]
		}
		cur[0] = i
		low := i
		for j := 1; j <= len(b); j++ {
			bj := b[j-1]
			cost := 1
			if ai == bj {
				cost = 0
			}
			v := min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
			if ai == last && before == bj {
				v = min(v, prev2[j-2]+1)
			}
			cur[j] = v
			low = min(low, v)
			last = bj
		}
		if low > k {
			return false
		}
		prev2, prev, cur = prev, cur, prev2
	}
	for j := max(len(a)-k, 0); j <= len(b); j++ {
		if prev[j] <= k {
			return true
		}
	}
	return false
}
