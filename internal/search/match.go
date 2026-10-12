package search

import (
	"slices"
	"strings"
	"sync"
	"unicode"

	"github.com/junegunn/fzf/src/algo"
	"github.com/junegunn/fzf/src/util"
)

const (
	slab16 = 16 * 1024
	slab32 = 2048
)

var _ = algo.Init("default")

var slabs = sync.Pool{New: func() any { return util.MakeSlab(slab16, slab32) }}

type term struct {
	text  []rune
	fold  bool
	not   bool
	fuzzy bool
	algo  algo.Algo
}

func newTerm(word string) term {
	t := term{algo: algo.FuzzyMatchV2}
	word, t.not = cutPrefix(word, "!")
	if t.not {
		t.algo = algo.ExactMatchNaive
	}
	word, exact := cutPrefix(word, "'")
	word, prefix := cutPrefix(word, "^")
	word, suffix := cutSuffix(word, "$")
	switch {
	case prefix && suffix:
		t.algo = algo.EqualMatch
	case prefix:
		t.algo = algo.PrefixMatch
	case suffix:
		t.algo = algo.SuffixMatch
	case exact:
		t.algo = algo.ExactMatchNaive
	}
	t.fuzzy = !t.not && !exact && !prefix && !suffix
	t.fold = !strings.ContainsFunc(word, unicode.IsUpper)
	r := []rune(word)
	if t.fold {
		for i, c := range r {
			r[i] = unicode.ToLower(c)
		}
	}
	t.text = algo.NormalizeRunes(r)
	return t
}

func cutPrefix(word, mark string) (string, bool) {
	if rest, ok := strings.CutPrefix(word, mark); ok && rest != "" {
		return rest, true
	}
	return word, false
}

func cutSuffix(word, mark string) (string, bool) {
	if rest, ok := strings.CutSuffix(word, mark); ok && rest != "" {
		return rest, true
	}
	return word, false
}

func terms(text string) []term {
	words := strings.Fields(text)
	out := make([]term, len(words))
	for i, w := range words {
		out[i] = newTerm(w)
	}
	return out
}

func (t *term) match(c *util.Chars, withPos bool, slab *util.Slab) (int, *[]int) {
	res, pos := t.algo(!t.fold, true, true, c, t.text, withPos, slab)
	if res.Start < 0 {
		return 0, nil
	}
	return max(res.Score, 1), pos
}

func Positions(query, s string) []int {
	words := terms(parseFilter(query).Text)
	if len(words) == 0 || s == "" {
		return nil
	}
	c := util.ToChars([]byte(s))
	slab := slabs.Get().(*util.Slab)
	defer slabs.Put(slab)
	var out []int
	for i := range words {
		if words[i].not {
			continue
		}
		if _, pos := words[i].match(&c, true, slab); pos != nil {
			out = append(out, *pos...)
		} else if from, to, ok := words[i].typo(&c); ok {
			for at := from; at < to; at++ {
				out = append(out, at)
			}
		}
	}
	slices.Sort(out)
	return slices.Compact(out)
}
